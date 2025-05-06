package service

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"time"

	"github.com/chain4travel/camino-messenger-bot/pkg/chequehandler"
	cmaccounts "github.com/chain4travel/camino-messenger-bot/pkg/cm_accounts"
	"github.com/chain4travel/camino-messenger-bot/pkg/matrix"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const (
	networkFee           uint64 = 100000 // nCAM
	cashInTxIssueTimeout        = 10 * time.Second
	cmAccountsCacheSize         = 100
)

var (
	_ Service = (*service)(nil)

	ErrNotFound = errors.New("not found")
)

type Service interface {
	ProcessEvents(ctx context.Context, events []event.Event) error
}

func NewService(
	ctx context.Context,
	logger *zap.SugaredLogger,
	contractAddr common.Address,
	networkFeeRecipientKey *ecdsa.PrivateKey,
	minDurationUntilExpiration uint64,
	storage Storage,
	ethClient *ethclient.Client,
	chainID *big.Int,
	cmAccounts cmaccounts.Service,
) (Service, error) {
	return &service{
		logger:                     logger,
		ethClient:                  ethClient,
		networkFeeRecipientKey:     networkFeeRecipientKey,
		networkFeeRecipientAddress: contractAddr,
		storage:                    storage,
		chainID:                    chainID,
		minDurationUntilExpiration: big.NewInt(0).SetUint64(minDurationUntilExpiration),
		cmAccounts:                 cmAccounts,
	}, nil
}

type service struct {
	logger                     *zap.SugaredLogger
	ethClient                  *ethclient.Client
	storage                    Storage
	networkFeeRecipientKey     *ecdsa.PrivateKey
	networkFeeRecipientAddress common.Address
	chainID                    *big.Int
	minDurationUntilExpiration *big.Int
	cmAccounts                 cmaccounts.Service
	chequeHandler              chequehandler.ChequeHandler
}

func (s *service) ProcessEvents(ctx context.Context, events []event.Event) error {
	for _, evnt := range events {
		if evnt.Type.Type == matrix.EventTypeC4TMessage.Type {
			if err := evnt.Content.ParseRaw(matrix.EventTypeC4TMessage); err != nil {
				s.logger.Errorf("Failed to parse event content: %v", err)
				continue
			}

			msg, ok := evnt.Content.Parsed.(*matrix.CaminoMatrixMessage)
			if !ok {
				err := errors.New("unexpected event content type")
				s.logger.Error(err)
				continue
			}

			banSender, err := s.processMessage(ctx, msg, evnt.ID)
			if err != nil {
				return err
			}

			if banSender {
				if err := s.banUser(ctx, evnt.Sender); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// processMessage extracts network fee cheque, verifies it and stores it in the database.
// Returns true if cheque is not valid or not covering all message chunks, indicating that sender should be banned
func (s *service) processMessage(ctx context.Context, msg *matrix.CaminoMatrixMessage, eventID id.EventID) (bool, error) {
	s.logger.Debugf("Processing message %s...", eventID)
	defer s.logger.Debugf("Finished message %s", eventID)

	if err := msg.Metadata.Verify(); err != nil {
		s.logger.Debugf("Invalid message metadata: %v", err)
		return false, err
	}

	session, err := s.storage.NewSession(ctx)
	if err != nil {
		s.logger.Errorf("Couldn't create storage session: %v", err)
		return false, err
	}
	defer session.Abort()

	switch {
	case msg.Metadata.ChunkIndex == 0:
		cheque := msg.GetChequeFor(s.networkFeeRecipientAddress)
		if cheque == nil {
			s.logger.Infof("event (%s) does not contain cheque for ASB owner %s", eventID, s.networkFeeRecipientAddress)
			return true, nil
		}

		if _, _, err := s.storage.GetChunksNumbers(ctx, session, msg.Metadata.RequestID); !errors.Is(err, ErrNotFound) { // TODO@ remove this check
			s.logger.Infof("Dropping message first chunk: %v", err) // TODO@ already exist or error
			return true, err
		}

		// TODO@ sender must be bot
		if err := s.chequeHandler.VerifyCheque(ctx, cheque, common.HexToAddress(msg.Metadata.Sender), nil); err != nil {
			s.logger.Infof("Failed to verify cheque: %v", err)
			return true, nil
		}

		if err := s.storage.InsertChunkedMessage(ctx, session, msg.Metadata.RequestID, msg.Metadata.NumberOfChunks); err != nil { // TODO@ insert, not upsert; error on conflict
			s.logger.Errorf("Failed to store message chunk: %v", err)
			return false, err
		}

		return false, session.Commit()

	case msg.Metadata.ChunkIndex == msg.Metadata.NumberOfChunks-1:
		if err := s.storage.DeleteChunkedMessage(ctx, session, msg.Metadata.RequestID); err != nil { // TODO@ delete, error on not found
			s.logger.Errorf("Failed to delete message chunk: %v", err)
			return false, err
		}

		return false, session.Commit()

	default: // Middle chunk, first chunk is already stored
		storedChunksNumber, maxChunksNumber, err := s.storage.GetChunksNumbers(ctx, session, msg.Metadata.RequestID)
		if err != nil {
			s.logger.Errorf("Couldn't create storage session: %v", err)
			return false, err
		}

		if storedChunksNumber == 0 { // TODO@ should never happen ?
			s.logger.Infof("event (%s) is not the first chunk, but no first chunk stored", eventID)
			return true, nil
		}

		if msg.Metadata.ChunkIndex > maxChunksNumber-1 {
			s.logger.Infof("event (%s) chunk index %d is out of range (0-%d)", eventID, msg.Metadata.ChunkIndex, maxChunksNumber-1)
			return true, nil
		}

		if err := s.storage.AddMessageChunk(ctx, session, msg.Metadata.RequestID); err != nil { // TODO@ update, not upsert; error on not found
			s.logger.Errorf("Failed to add message chunk: %v", err)
			return false, err
		}

		return false, session.Commit()
	}
	// TODO @evlekht do cash in amount threshold reached? store unpaid amount in cheque?
}

// TODO @evlekht implement (next ticket) // persist with db, make it durable? not just call it from event receiver?
func (s *service) banUser(_ context.Context, _ id.UserID) error {
	return nil
}

func waitMined(ctx context.Context, b bind.DeployBackend, txID common.Hash) (*types.Receipt, error) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		receipt, err := b.TransactionReceipt(ctx, txID)
		if err == nil {
			return receipt, nil
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}
