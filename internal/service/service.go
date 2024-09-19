package service

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"net/url"
	"time"

	"github.com/chain4travel/camino-messenger-bot/pkg/cheques"
	"github.com/chain4travel/camino-messenger-bot/pkg/matrix"
	"github.com/chain4travel/camino-messenger-contracts/go/contracts/cmaccount"
	"github.com/chain4travel/camino-synapse-app-service/internal/logger"
	"github.com/chain4travel/camino-synapse-app-service/internal/models"
	"github.com/chain4travel/camino-synapse-app-service/internal/storage"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	lru "github.com/hashicorp/golang-lru/v2"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const networkFee uint64 = 100000 // nCAM

var _ Service = (*service)(nil)

type Service interface {
	ProcessEvents(ctx context.Context, events []event.Event) error
	CashIn(ctx context.Context) error
}

func NewService(
	ctx context.Context,
	logger logger.Logger,
	nodeURI url.URL,
	contractAddr common.Address,
	networkFeeRecipientKey *ecdsa.PrivateKey,
	minDurationUntilExpiration uint64,
	storage storage.Storage,
) (Service, error) {
	ethClient, err := ethclient.Dial(nodeURI.String() + "/ext/bc/C/rpc")
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	transactor, err := bind.NewKeyedTransactorWithChainID(networkFeeRecipientKey, chainID)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	cmAccountsCache, err := lru.New[common.Address, *cmaccount.Cmaccount](10)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	return &service{
		logger:                     logger,
		ethClient:                  ethClient,
		networkFeeRecipientKey:     networkFeeRecipientKey,
		networkFeeRecipientAddress: contractAddr,
		storage:                    storage,
		transactor:                 transactor,
		minDurationUntilExpiration: big.NewInt(0).SetUint64(minDurationUntilExpiration),
		cmAccounts:                 cmAccountsCache,
	}, nil
}

type service struct {
	logger                     logger.Logger
	ethClient                  *ethclient.Client
	storage                    storage.Storage
	networkFeeRecipientKey     *ecdsa.PrivateKey
	networkFeeRecipientAddress common.Address
	transactor                 *bind.TransactOpts
	minDurationUntilExpiration *big.Int
	cmAccounts                 *lru.Cache[common.Address, *cmaccount.Cmaccount]
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

func (s *service) processMessage(ctx context.Context, msg *matrix.CaminoMatrixMessage, eventID id.EventID) (bool, error) {
	s.logger.Debugf("Processing message %s...", eventID)
	defer s.logger.Debug("Finished message %s")

	session, err := s.storage.NewSession(ctx)
	if err != nil {
		s.logger.Errorf("Couldn't create storage session: %v", err)
		return false, err
	}
	defer session.Abort()

	storedChunksNumber, maxChunksNumber, err := session.GetChunksNumbers(ctx, msg.Metadata.RequestID)
	if err != nil && errors.Is(err, storage.ErrNotFound) {
		s.logger.Errorf("Couldn't create storage session: %v", err)
		return false, err
	}

	if storedChunksNumber > 0 && msg.Metadata.ChunkIndex > 0 {
		// Middle chunk, first chunk is already stored
		if err := session.AddMessageChunk(ctx, msg.Metadata.RequestID); err != nil {
			s.logger.Errorf("Failed to add message chunk: %v", err)
			return false, err
		}
		return false, session.Commit()
	}

	cheque := msg.GetChequeFor(s.networkFeeRecipientAddress)
	if cheque == nil {
		s.logger.Infof("event (%s) does not contain cheque for ASB owner %s", eventID, s.networkFeeRecipientAddress)
		return true, nil
	}

	chequebookID := chequebookID(cheque)
	chequebook, err := session.GetChequebook(ctx, chequebookID)
	if err != nil && errors.Is(err, storage.ErrNotFound) {
		s.logger.Errorf("Failed to get cheque: %v", err)
		return false, err
	}

	var previousCheque *cheques.SignedCheque
	if chequebook != nil {
		previousCheque = &chequebook.SignedCheque
	}
	if err := cheques.VerifyCheque(
		previousCheque,
		cheque,
		big.NewInt(time.Now().Unix()),
		s.minDurationUntilExpiration,
	); err != nil {
		s.logger.Errorf("Failed to verify cheque: %v", err)
		return true, nil
	}

	if valid, err := s.verifyChequeWithContract(ctx, cheque); err != nil {
		s.logger.Errorf("Failed to verify cheque with blockchain: %v", err)
		return false, err
	} else if !valid {
		s.logger.Infof("cheque is invalid (blockchain validation)")
		return true, nil
	}

	if chequebook == nil {
		if err := session.AddChequebook(ctx, chequebookFromCheque(chequebookID, cheque)); err != nil {
			s.logger.Errorf("Failed to store cheque: %v", err)
			return false, err
		}
	} else {
		if err := session.UpdateChequebook(ctx, &models.Chequebook{
			SignedCheque: *cheque,
			ChequebookID: chequebookID,
		}); err != nil {
			s.logger.Errorf("Failed to store cheque: %v", err)
			return false, err
		}
	}

	expectedAmount := networkFee * msg.Metadata.NumberOfChunks
	if amount := cheque.Amount.Uint64(); amount < expectedAmount {
		s.logger.Infof("cheque contain enough fee for all chunks (has %d, need %d, fee-per-chunk %d, chunks %d)",
			amount, expectedAmount,
			networkFee, msg.Metadata.NumberOfChunks)
		return true, session.Commit()
	}

	ban := false
	switch {
	case storedChunksNumber == 0 && msg.Metadata.ChunkIndex != 0:
		// Not first chunk, but no first chunk stored
		s.logger.Infof("event (%s) is not the first chunk, but there is no first chunk stored", eventID)
		ban = true
	case storedChunksNumber == 0 && msg.Metadata.ChunkIndex == 0:
		// First chunk was received
		err = session.AddChunkedMessage(ctx, msg.Metadata.RequestID, msg.Metadata.NumberOfChunks)
	case storedChunksNumber == maxChunksNumber-1:
		// Last chunk was received
		err = session.DeleteChunkedMessage(ctx, msg.Metadata.RequestID)
	case msg.Metadata.ChunkIndex > 1:
		// Middle chunk was received
		err = session.AddMessageChunk(ctx, msg.Metadata.RequestID)
	}
	if err != nil {
		s.logger.Errorf("Failed to store message chunk: %v", err)
		return false, err
	}

	return ban, session.Commit()
}

func (s *service) verifyChequeWithContract(ctx context.Context, cheque *cheques.SignedCheque) (bool, error) {
	cmAccount, err := s.getCMAccount(cheque.FromCMAccount)
	if err != nil {
		s.logger.Errorf("failed to get cmAccount contract instance: %v", err)
		return false, err
	}
	_, err = cmAccount.VerifyCheque(
		&bind.CallOpts{Context: ctx},
		cheque.FromCMAccount,
		cheque.ToCMAccount,
		cheque.ToBot,
		cheque.Counter,
		cheque.Amount,
		cheque.CreatedAt,
		cheque.ExpiresAt,
		cheque.Signature,
	)
	if err != nil && err.Error() == "execution reverted" {
		return false, nil
	}
	return err == nil, err
}

// TODO @evlekht implement (next ticket) // persist with db, make it durable? not just call it from event receiver?
func (s *service) banUser(_ context.Context, _ id.UserID) error {
	return nil
}

func (s *service) CashIn(ctx context.Context) error {
	s.logger.Debug("Cashing in...")
	defer s.logger.Debug("Finished cashing in")

	session, err := s.storage.NewSession(ctx)
	if err != nil {
		s.logger.Error(err)
		return err
	}
	defer session.Abort()

	cheques, err := session.GetNotCashedChequebooks(ctx)
	if err != nil {
		s.logger.Errorf("failed to get not cashed cheques: %v", err)
		return err
	}

	for _, cheque := range cheques {
		s.logger.Debugf("Checking cheque %s status...", cheque)

		updateChequebook := false
		switch cheque.Status {
		case models.ChequeTxStatusUnknown, models.ChequeTxStatusRejected:
			cmAccount, err := s.getCMAccount(cheque.FromCMAccount)
			if err != nil {
				s.logger.Errorf("failed to get cmAccount contract instance: %v", err)
				continue
			}

			tx, err := cmAccount.CashInCheque(
				s.transactor, // TODO@ timeout context ?
				cheque.FromCMAccount,
				cheque.ToCMAccount,
				cheque.ToBot,
				cheque.Counter,
				cheque.Amount,
				cheque.CreatedAt,
				cheque.ExpiresAt,
				cheque.Signature,
			)
			if err != nil {
				s.logger.Errorf("failed to cash in cheque %s: %v", cheque, err)
				continue
			}

			// TODO@ its old comment, reverify
			// ! @evlekht if tx will be issued, but then storage will fail to persist it,
			// ! tx is still issued and app service will fail to cash in this cheque next time
			// ! cause on the node side it is already cashed in
			cheque.TxID = tx.Hash()
			cheque.Status = models.ChequeTxStatusProcessing
			updateChequebook = true
		case models.ChequeTxStatusProcessing:
			// TODO@ move to eth event listener // listen for all not-fully-cashed chequebooks, even ones without txs // also check status on startup
			res, err := s.ethClient.TransactionReceipt(ctx, cheque.TxID)
			if err != nil {
				s.logger.Errorf("failed to get cash in transaction receipt for cheque %s: %v", cheque, err)
				continue
			}

			txStatus := models.ChequeTxStatusRejected
			if res.Status == types.ReceiptStatusSuccessful {
				txStatus = models.ChequeTxStatusAccepted
			}

			updateChequebook = cheque.Status != txStatus
			cheque.Status = txStatus
		}

		if updateChequebook {
			if err := session.UpdateChequebook(ctx, &cheque); err != nil {
				s.logger.Errorf("failed to update cheque %s: %v", cheque, err)
				return nil
			}
		}
	}

	return session.Commit()
}

func (s *service) getCMAccount(address common.Address) (*cmaccount.Cmaccount, error) {
	cmAccount, ok := s.cmAccounts.Get(address)
	if ok {
		return cmAccount, nil
	}

	cmAccount, err := cmaccount.NewCmaccount(address, s.ethClient)
	if err != nil {
		s.logger.Errorf("failed to create cmAccount contract instance: %v", err)
		return nil, err
	}
	_ = s.cmAccounts.Add(address, cmAccount)

	return cmAccount, nil
}

func chequebookFromCheque(chequebookID common.Hash, cheque *cheques.SignedCheque) *models.Chequebook {
	return &models.Chequebook{
		SignedCheque: cheques.SignedCheque{
			Cheque: cheques.Cheque{
				FromCMAccount: cheque.FromCMAccount,
				ToCMAccount:   cheque.ToCMAccount,
				ToBot:         cheque.ToBot,
				Counter:       cheque.Counter,
				Amount:        cheque.Amount,
				CreatedAt:     cheque.CreatedAt,
				ExpiresAt:     cheque.ExpiresAt,
			},
			Signature: cheque.Signature,
		},
		ChequebookID: chequebookID,
	}
}

func chequebookID(cheque *cheques.SignedCheque) common.Hash {
	return crypto.Keccak256Hash(
		cheque.FromCMAccount.Bytes(),
		cheque.ToCMAccount.Bytes(),
		cheque.ToBot.Bytes(),
	)
}
