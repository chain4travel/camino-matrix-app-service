package service

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"math/big"
	"net/url"
	"sync"
	"time"

	"github.com/chain4travel/camino-matrix-app-service/internal/logger"
	"github.com/chain4travel/camino-matrix-app-service/internal/models"
	"github.com/chain4travel/camino-matrix-app-service/internal/storage"
	"github.com/chain4travel/camino-messenger-bot/pkg/cheques"
	"github.com/chain4travel/camino-messenger-bot/pkg/matrix"
	"github.com/chain4travel/camino-messenger-contracts/go/contracts/cmaccount"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	lru "github.com/hashicorp/golang-lru/v2"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const (
	networkFee           uint64 = 100000 // nCAM
	cashInTxIssueTimeout        = 10 * time.Second
	cmAccountsCacheSize         = 100
)

var _ Service = (*service)(nil)

type Service interface {
	ProcessEvents(ctx context.Context, events []event.Event) error
	CashIn(ctx context.Context) error
	CheckCashInStatus(ctx context.Context) error
}

func NewService(
	ctx context.Context,
	logger logger.Logger,
	cChainRPCURL url.URL,
	contractAddr common.Address,
	networkFeeRecipientKey *ecdsa.PrivateKey,
	minDurationUntilExpiration uint64,
	storage storage.Storage,
) (Service, error) {
	ethClient, err := ethclient.Dial(cChainRPCURL.String())
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	chainID, err := ethClient.ChainID(ctx)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	cmAccountsCache, err := lru.New[common.Address, *cmaccount.Cmaccount](cmAccountsCacheSize)
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
		chainID:                    chainID,
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
	chainID                    *big.Int
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

// processMessage extracts network fee cheque, verifies it and stores it in the database.
// Returns true if cheque is not valid or not covering all message chunks, indicating that sender should be banned
func (s *service) processMessage(ctx context.Context, msg *matrix.CaminoMatrixMessage, eventID id.EventID) (bool, error) {
	s.logger.Debugf("Processing message %s...", eventID)
	defer s.logger.Debugf("Finished message %s", eventID)

	session, err := s.storage.NewSession(ctx)
	if err != nil {
		s.logger.Errorf("Couldn't create storage session: %v", err)
		return false, err
	}
	defer session.Abort()

	storedChunksNumber, maxChunksNumber, err := session.GetChunksNumbers(ctx, msg.Metadata.RequestID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
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
	s.logger.Debugf("Received cheque %s %+v", chequebookID, cheque.Cheque)

	chequebook, err := session.GetChequebook(ctx, chequebookID)
	if err != nil && !errors.Is(err, storage.ErrNotFound) {
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
		s.logger.Infof("Failed to verify cheque: %v", err)
		return true, nil
	}

	if valid, err := s.verifyChequeWithContract(ctx, cheque); err != nil {
		s.logger.Errorf("Failed to verify cheque with blockchain: %v", err)
		return false, err
	} else if !valid {
		s.logger.Infof("cheque is invalid (blockchain validation)")
		return true, nil
	}

	chequebook = chequebookFromCheque(chequebookID, cheque)
	if err := session.UpsertChequebook(ctx, chequebook); err != nil {
		s.logger.Errorf("Failed to store cheque: %v", err)
		return false, err
	}

	expectedAmount := networkFee * msg.Metadata.NumberOfChunks
	if amount := cheque.Amount.Uint64(); amount < expectedAmount {
		s.logger.Infof("cheque contain enough fee for all chunks (has %d, need %d, fee-per-chunk %d, chunks %d)",
			amount, expectedAmount,
			networkFee, msg.Metadata.NumberOfChunks)
		return true, session.Commit()
	}

	err = nil
	ban := false
	switch {
	case storedChunksNumber == 0 && msg.Metadata.ChunkIndex != 0:
		// Not first chunk, but no first chunk stored
		s.logger.Infof("event (%s) is not the first chunk, but there is no first chunk stored", eventID)
		ban = true
	case storedChunksNumber == 0 && msg.Metadata.ChunkIndex == 0 && msg.Metadata.NumberOfChunks > 1:
		// First chunk of multi-chunk message was received
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

	// TODO @evlekht do cash in amount threshold reached? store unpaid amount in cheque?

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

	chequebooks, err := session.GetNotCashedChequebooks(ctx)
	if err != nil {
		s.logger.Errorf("failed to get not cashed cheques: %v", err)
		return err
	}

	wg := sync.WaitGroup{}
	for _, chequebook := range chequebooks {
		s.logger.Debugf("Checking cheque %s status...", chequebook)

		wg.Add(1)
		go func() {
			defer wg.Done()

			cmAccount, err := s.getCMAccount(chequebook.FromCMAccount)
			if err != nil {
				s.logger.Errorf("failed to get cmAccount contract instance: %v", err)
				return
			}

			transactor, err := bind.NewKeyedTransactorWithChainID(s.networkFeeRecipientKey, s.chainID)
			if err != nil {
				s.logger.Error(err)
				return
			}

			timedCtx, cancel := context.WithTimeout(ctx, cashInTxIssueTimeout)
			defer cancel()
			transactor.Context = timedCtx

			tx, err := cmAccount.CashInCheque(
				transactor,
				chequebook.FromCMAccount,
				chequebook.ToCMAccount,
				chequebook.ToBot,
				chequebook.Counter,
				chequebook.Amount,
				chequebook.CreatedAt,
				chequebook.ExpiresAt,
				chequebook.Signature,
			)
			if err != nil {
				s.logger.Errorf("failed to cash in cheque %s: %v", chequebook, err)
				return
			}

			txID := tx.Hash()
			chequebook.TxID = txID
			chequebook.Status = models.ChequeTxStatusProcessing

			// TODO @evlekht if tx will be issued, but then storage will fail to persist it,
			// TODO tx is still issued and app service will fail to cash in this cheque next time
			// TODO cause on the node side it is already cashed in
			// TODO possible solution would be to do dry run, get txID, commit session with txID and status "processing",
			// TODO then do real run? also do same on startup

			// TODO @evlekht add txCreatedAt field to db and use it for mining timeout ?

			if err := session.UpsertChequebook(ctx, chequebook); err != nil {
				s.logger.Errorf("failed to update cheque %s: %v", chequebook, err)
				return
			}
		}()
	}

	wg.Wait()

	if err := session.Commit(); err != nil {
		s.logger.Errorf("failed to commit session: %v", err)
		return err
	}

	for _, chequebook := range chequebooks {
		txID := chequebook.TxID
		go func() {
			_ = s.checkCashInStatus(context.Background(), txID)
		}()
	}

	return nil
}

func (s *service) CheckCashInStatus(ctx context.Context) error {
	session, err := s.storage.NewSession(ctx)
	if err != nil {
		s.logger.Error(err)
		return err
	}
	defer session.Abort()

	chequebooks, err := session.GetChequebooksWithPendingTxs(ctx)
	if err != nil {
		s.logger.Errorf("failed to get not cashed cheques: %v", err)
		return err
	}

	for _, chequebook := range chequebooks {
		txID := chequebook.TxID
		go func() {
			_ = s.checkCashInStatus(ctx, txID)
		}()
	}

	return nil
}

func (s *service) checkCashInStatus(ctx context.Context, txID common.Hash) error {
	// TODO @evlekht timeout? what to do if timeouted?
	res, err := waitMined(ctx, s.ethClient, txID)
	if err != nil {
		s.logger.Errorf("failed to get cash in transaction receipt %s: %v", txID, err)
		return err
	}

	session, err := s.storage.NewSession(ctx)
	if err != nil {
		s.logger.Error(err)
		return err
	}
	defer session.Abort()

	chequebook, err := session.GetChequebookByTxID(ctx, txID)
	if err != nil {
		s.logger.Errorf("failed to get chequebook by txID %s: %v", txID, err)
		return err
	}

	txStatus := models.ChequeTxStatusFromTxStatus(res.Status)
	if chequebook.Status == txStatus {
		return nil
	}

	chequebook.Status = txStatus
	if err := session.UpsertChequebook(ctx, chequebook); err != nil {
		s.logger.Errorf("failed to update chequebook %s: %v", chequebook, err)
		return err
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
