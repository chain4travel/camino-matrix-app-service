package service

import (
	"context"
	"fmt"

	"github.com/chain4travel/camino-synapse-app-service/internal/matrix"
	"github.com/chain4travel/camino-synapse-app-service/internal/models"
	"github.com/chain4travel/camino-synapse-app-service/internal/node"
	"github.com/chain4travel/camino-synapse-app-service/internal/storage"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/hashing"
	tStatus "github.com/ava-labs/avalanchego/vms/touristicvm/status"
	"go.uber.org/zap"
	"maunium.net/go/mautrix/event"
)

func New(
	ctx context.Context,
	logger *zap.SugaredLogger,
	storage storage.Storage,
	nodeClient node.Client,
	matrixClient matrix.Client,
) *Service {
	return &Service{
		parentCtx:    ctx,
		logger:       logger,
		storage:      storage,
		nodeClient:   nodeClient,
		matrixClient: matrixClient,
	}
}

type Service struct {
	logger           *zap.SugaredLogger
	nodeClient       node.Client
	matrixClient     matrix.Client
	storage          storage.Storage
	secp256k1Factory secp256k1.Factory
	parentCtx        context.Context
}

func (s *Service) ProcessEvents(ctx context.Context, events []event.Event, matrixTxID string) error {
	s.logger.Debug("Processing matrix events...")
	defer s.logger.Debug("Finished processing matrix events")
	session, err := s.storage.NewSession(context.Background())
	if err != nil {
		return err
	}
	defer session.Abort()

	for _, evnt := range events {
		s.logger.Debugf("Got event: %s", evnt.Type.Type)
		if matrix.IsC4TMessage(&evnt) {
			cheques, err := s.matrixClient.GetC4TMessageCheques(evnt)
			if err != nil {
				return err
			}
			for i, cheque := range cheques {
				if err := cheque.Verify(); err != nil {
					s.logger.Infof("txnId:%s[%d] cheque (%s) is syntactically invalid: %v", matrixTxID, i, cheque, err)
					continue
				}

				if err := s.verifyChequeSignature(&cheque); err != nil {
					s.logger.Infof("txnId:%s[%d] cheque (%s) signature is invalid: %v", matrixTxID, i, cheque, err)
					continue
				}

				previousChequeRecord, err := session.GetCheque(ctx, cheque.ChequebookID)
				if err != nil && err != storage.ErrNotFound {
					return err
				}
				if err := cheque.VerifyWithPrevious(previousChequeRecord); err != nil {
					s.logger.Infof("txnId:%s[%d] cheque (%s) is semantically invalid: %v", matrixTxID, i, cheque, err)
					continue
				}

				if previousChequeRecord == nil {
					if err := session.AddCheque(ctx, &cheque); err != nil {
						return err
					}
				} else {
					if err := session.UpdateCheque(ctx, &cheque); err != nil {
						return err
					}
				}

				s.logger.Debugf("Added cheque %s", cheque)
			}
		}
	}

	if err := session.Commit(); err != nil {
		s.logger.Error(err)
		return err
	}

	return nil
}

func (s *Service) CashOut() {
	s.logger.Debug("Issuing cashOutTxs...")
	defer s.logger.Debug("Finished issuing cashOutTxs")
	session, err := s.storage.NewSession(context.Background())
	if err != nil {
		return
	}
	defer session.Abort()

	cheques, err := session.GetNotCashedCheques(s.parentCtx)
	if err != nil {
		s.logger.Errorf("failed to get not cashed cheques: %v", err)
		return
	}

	for _, cheque := range cheques {
		s.logger.Debugf("Checking cheque %s status...", cheque)
		update := false

		if cheque.TxID != ids.Empty && (cheque.Status == tStatus.Unknown || cheque.Status == tStatus.Processing) {
			txStatus, _, err := s.nodeClient.GetTxStatus(cheque.TxID)
			if err != nil {
				s.logger.Infof("failed to get cashOutTx status for cheque %s: %v", cheque, err)
				continue
			}
			update = cheque.Status != txStatus
			cheque.Status = txStatus
		}

		if cheque.TxID == ids.Empty || cheque.Status == tStatus.Aborted || cheque.Status == tStatus.Dropped {
			txID, err := s.nodeClient.CashOutTx(&cheque)
			if err != nil {
				s.logger.Infof("failed to issue cashOutTx for cheque %s: %v", cheque, err)
				continue
			}
			// ! @evlekht if tx will be issued, but then storage will fail to persist it,
			// ! tx is still issued and app service will fail to cash out this cheque next time
			// ! cause on the node side it is already cashed out
			cheque.TxID = txID
			cheque.Status = tStatus.Unknown
			update = true
		}

		if update {
			if err := session.UpdateCheque(s.parentCtx, &cheque); err != nil {
				s.logger.Errorf("failed to update cheque %s: %v", cheque, err)
				return
			}
		}
	}

	if err := session.Commit(); err != nil {
		s.logger.Error(err)
	}
}

func (s *Service) verifyChequeSignature(cheque *models.Cheque) error {
	chequeHash := hashing.ComputeHash256(cheque.Cheque.BuildMsgToSign())
	pubKey, err := s.secp256k1Factory.RecoverHashPublicKey(chequeHash, cheque.Signature[:])
	if err != nil {
		return fmt.Errorf("failed to recover public key from cheque signature: %v", err)
	}

	if pubKey.Address() != cheque.Issuer {
		return fmt.Errorf("signed by %s, but expected %s", pubKey.Address().String(), cheque.Issuer.String())
	}

	return nil
}
