package service

import (
	"context"

	"github.com/chain4travel/camino-synapse-app-service/internal/matrix"
	"github.com/chain4travel/camino-synapse-app-service/internal/node"
	"github.com/chain4travel/camino-synapse-app-service/internal/storage"

	"github.com/ava-labs/avalanchego/vms/touristicvm/status"
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
	return &Service{logger: logger, storage: storage, nodeClient: nodeClient, matrixClient: matrixClient}
}

type Service struct {
	logger       *zap.SugaredLogger
	nodeClient   node.Client
	matrixClient matrix.Client
	storage      storage.Storage
}

func (s *Service) ProcessEvents(events []event.Event, matrixTxID string) error {
	s.logger.Debugf("Processing events (awaiting new storage session)...")
	s.storage.NewSession()
	defer s.storage.Abort()
	s.logger.Debugf("Processing events...")

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
				previousChequeRecord, err := s.GetLastAddedChequeRecord(&cheque)
				if err != nil {
					return err
				}
				if err := cheque.VerifyWithPrevious(previousChequeRecord); err != nil {
					s.logger.Infof("txnId:%s[%d] cheque (%s) is semantically invalid: %v", matrixTxID, i, cheque, err)
					continue
				}
				if err := s.AddCheque(&cheque); err != nil {
					return err
				}
				s.logger.Debugf("Added cheque %s", &cheque)
			}
		}
	}
	s.logger.Debugf("Finished processing events")
	if err := s.storage.Commit(); err != nil {
		s.logger.Error(err)
	}
	return nil
}

func (s *Service) CashOut() {
	s.logger.Debugf("Issuing cashOutTxs (awaiting new storage session)...")
	s.storage.NewSession()
	defer s.storage.Abort()
	s.logger.Debugf("Issuing cashOutTxs...")

	it := s.storage.GetChequebooksIterator()
	defer it.Release()
	for it.Next() {
		chequebook, err := it.Value()
		if err != nil {
			s.logger.Error(err)
			continue
		}
		if chequebook.IsCashedOut() {
			continue
		}

		cheque := chequebook.LastAddedCheque()
		s.logger.Debugf("Cashing out cheque %s ...", cheque)

		txID, err := s.nodeClient.CashOutTx(cheque)
		if err != nil {
			s.logger.Infof("failed to issue cashOutTx for cheque %+v: %v", cheque, err)
			continue
		}

		if err := s.storage.AddChequeTx(txID, cheque); err != nil {
			s.logger.Errorf("failed to add cheque %s tx %s: %v", cheque, txID, err)
			return
		}
	}
	if err := it.Error(); err != nil {
		s.logger.Error(err)
	}
	s.logger.Debugf("Finished issuing cashOutTxs")
	if err := s.storage.Commit(); err != nil {
		s.logger.Error(err)
	}
}

func (s *Service) CheckCashOutTxs() {
	s.logger.Debugf("Checking cashOutTxs (awaiting new storage session)...")
	s.storage.NewSession()
	defer s.storage.Abort()
	s.logger.Debugf("Checking cashOutTxs...")

	it := s.storage.GetChequeTxsIterator()
	defer it.Release()
	for it.Next() {
		txID, cheque, err := it.Value()
		if err != nil { // should never happen
			s.logger.Error(err)
			continue
		}
		s.logger.Debugf("Checking cashOutTx %s for cheque %s", txID, cheque)

		txStatus, txStatusReason, err := s.nodeClient.GetTxStatus(txID)
		if err != nil {
			s.logger.Error(err)
			continue
		}

		switch txStatus {
		case status.Unknown:
		case status.Aborted:
		case status.Dropped:
			s.logger.Debugf("Cheque %s tx is rejected with status %s (%s)", cheque, txStatus, txStatusReason)
			if err := s.storage.RemoveChequeTx(txID); err != nil {
				s.logger.Error(err)
				continue
			}
		case status.Processing:
			s.logger.Debugf("Cheque %s is not cashed out yet, tx status: %s", cheque, txStatus)
			continue
		case status.Committed:
			if err := s.storage.RemoveChequeTx(txID); err != nil {
				s.logger.Error(err)
				continue
			}

			if err := s.AddSuccessfulCashOut(cheque); err != nil {
				s.logger.Error(err)
				continue
			}
			s.logger.Debugf("Cheque %s is cashed out", cheque)
		default:
			s.logger.Warn("Unknown tx status %s (%s) for cheque %s", txStatus, cheque, txStatusReason)
		}

	}
	if err := it.Error(); err != nil {
		s.logger.Error(err)
	}
	s.logger.Debugf("Finished checking cashOutTxs")
	if err := s.storage.Commit(); err != nil {
		s.logger.Error(err)
	}
}
