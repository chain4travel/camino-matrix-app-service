package service

import (
	"github.com/chain4travel/camino-synapse-app-service/internal/models"

	"github.com/ava-labs/avalanchego/database"
)

func (s *Service) GetLastAddedChequeRecord(cheque *models.SignedCheque) (*models.ChequeRecord, error) {
	trio, err := s.storage.GetTrio(cheque.TrioID())
	switch {
	case err == database.ErrNotFound:
		return nil, nil
	case err != nil:
		s.logger.Error(err)
		return nil, err
	}
	return &trio.LastAdded, nil
}

func (s *Service) AddCheque(cheque *models.SignedCheque) error {
	trioID := cheque.TrioID()
	trio, err := s.storage.GetTrio(trioID)
	switch {
	case err == database.ErrNotFound:
		trio = cheque.Trio()
	case err != nil:
		s.logger.Error(err)
		return err
	}
	trio.LastAdded = cheque.ChequeRecord()
	return s.storage.SetTrio(trioID, trio)
}

func (s *Service) SetChequeIssuedWithTx(cheque *models.SignedCheque) error {
	trioID := cheque.TrioID()
	trio, err := s.storage.GetTrio(trioID)
	if err != nil {
		s.logger.Error(err)
		return err
	}

	if cheque.IsNewerThan(&trio.LastIssuedWithTx) {
		trio.LastIssuedWithTx = cheque.ChequeRecord()
	}

	return s.storage.SetTrio(trioID, trio)
}

func (s *Service) AddSuccessfulCashOut(cheque *models.SignedCheque) error {
	trioID := cheque.TrioID()
	trio, err := s.storage.GetTrio(trioID)
	if err != nil {
		s.logger.Error(err)
		return err
	}

	if cheque.IsNewerThan(&trio.LastCashedOut) {
		trio.LastCashedOut = cheque.ChequeRecord()
	}

	return s.storage.SetTrio(trioID, trio)
}
