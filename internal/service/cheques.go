package service

import (
	"github.com/chain4travel/camino-synapse-app-service/internal/models"

	"github.com/ava-labs/avalanchego/database"
)

func (s *Service) GetLastAddedChequeRecord(cheque *models.SignedCheque) (*models.ChequeRecord, error) {
	chequebook, err := s.storage.GetChequebook(cheque.ChequebookID())
	switch {
	case err == database.ErrNotFound:
		return nil, nil
	case err != nil:
		s.logger.Error(err)
		return nil, err
	}
	return &chequebook.LastAdded, nil
}

func (s *Service) AddCheque(cheque *models.SignedCheque) error {
	chequebookID := cheque.ChequebookID()
	chequebook, err := s.storage.GetChequebook(chequebookID)
	switch {
	case err == database.ErrNotFound:
		chequebook = cheque.Chequebook()
	case err != nil:
		s.logger.Error(err)
		return err
	}
	chequebook.LastAdded = cheque.ChequeRecord()
	return s.storage.SetChequebook(chequebookID, chequebook)
}

func (s *Service) SetChequeIssuedWithTx(cheque *models.SignedCheque) error {
	chequebookID := cheque.ChequebookID()
	chequebook, err := s.storage.GetChequebook(chequebookID)
	if err != nil {
		s.logger.Error(err)
		return err
	}

	if cheque.IsNewerThan(&chequebook.LastIssuedWithTx) {
		chequebook.LastIssuedWithTx = cheque.ChequeRecord()
	}

	return s.storage.SetChequebook(chequebookID, chequebook)
}

func (s *Service) AddSuccessfulCashOut(cheque *models.SignedCheque) error {
	chequebookID := cheque.ChequebookID()
	chequebook, err := s.storage.GetChequebook(chequebookID)
	if err != nil {
		s.logger.Error(err)
		return err
	}

	if cheque.IsNewerThan(&chequebook.LastCashedOut) {
		chequebook.LastCashedOut = cheque.ChequeRecord()
	}

	return s.storage.SetChequebook(chequebookID, chequebook)
}
