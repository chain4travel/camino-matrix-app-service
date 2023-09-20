package storage

import (
	"github.com/chain4travel/camino-synapse-app-service/internal/models"

	"github.com/ava-labs/avalanchego/database"
	"go.uber.org/zap"
)

func (s *storage) GetTrio(trioID string) (*models.Trio, error) {
	bytes, err := s.chequesDB.Get([]byte(trioID))
	if err != nil {
		return nil, err
	}
	record := &models.Trio{}
	if _, err := codec.Unmarshal(bytes, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *storage) SetTrio(trioID string, record *models.Trio) error {
	bytes, err := codec.Marshal(codecVersion, record)
	if err != nil {
		s.logger.Error(err)
		return err
	}
	if err := s.chequesDB.Put([]byte(trioID), bytes); err != nil {
		s.logger.Error(err)
		return err
	}
	return nil
}

func (s *storage) GetTriosIterator() ChequeTriosIterator {
	return &chequeTriosIterator{Iterator: s.chequesDB.NewIterator(), logger: s.logger}
}

type ChequeTriosIterator interface {
	Next() bool
	Error() error
	Value() (*models.Trio, error)
	Release()
}

var _ ChequeTriosIterator = (*chequeTriosIterator)(nil)

type chequeTriosIterator struct {
	logger *zap.SugaredLogger
	database.Iterator
}

func (it *chequeTriosIterator) Value() (*models.Trio, error) {
	value := it.Iterator.Value()
	if value == nil {
		return nil, nil
	}
	record := &models.Trio{}
	if _, err := codec.Unmarshal(value, record); err != nil {
		it.logger.Error(err)
		return nil, err
	}
	return record, nil
}
