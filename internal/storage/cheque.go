package storage

import (
	"github.com/chain4travel/camino-synapse-app-service/internal/models"

	"github.com/ava-labs/avalanchego/database"
	"go.uber.org/zap"
)

func (s *storage) GetChequebook(chequebookID string) (*models.Chequebook, error) {
	bytes, err := s.chequebooksDB.Get([]byte(chequebookID))
	if err != nil {
		return nil, err
	}
	record := &models.Chequebook{}
	if _, err := codec.Unmarshal(bytes, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *storage) SetChequebook(chequebookID string, record *models.Chequebook) error {
	bytes, err := codec.Marshal(codecVersion, record)
	if err != nil {
		s.logger.Error(err)
		return err
	}
	if err := s.chequebooksDB.Put([]byte(chequebookID), bytes); err != nil {
		s.logger.Error(err)
		return err
	}
	return nil
}

func (s *storage) GetChequebooksIterator() ChequebooksIterator {
	return &chequebooksIterator{Iterator: s.chequebooksDB.NewIterator(), logger: s.logger}
}

type ChequebooksIterator interface {
	Next() bool
	Error() error
	Value() (*models.Chequebook, error)
	Release()
}

var _ ChequebooksIterator = (*chequebooksIterator)(nil)

type chequebooksIterator struct {
	logger *zap.SugaredLogger
	database.Iterator
}

func (it *chequebooksIterator) Value() (*models.Chequebook, error) {
	value := it.Iterator.Value()
	if value == nil {
		return nil, nil
	}
	record := &models.Chequebook{}
	if _, err := codec.Unmarshal(value, record); err != nil {
		it.logger.Error(err)
		return nil, err
	}
	return record, nil
}
