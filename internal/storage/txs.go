package storage

import (
	"github.com/chain4travel/camino-synapse-app-service/internal/models"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/zap"
)

func (s *storage) RemoveChequeTx(txID ids.ID) error {
	if err := s.chequeTxsDB.Delete(txID[:]); err != nil {
		s.logger.Error(err)
		return err
	}
	return nil
}

func (s *storage) AddChequeTx(txID ids.ID, cheque *models.SignedCheque) error {
	bytes, err := codec.Marshal(codecVersion, cheque)
	if err != nil {
		s.logger.Error(err)
		return err
	}
	if err := s.chequeTxsDB.Put(txID[:], bytes); err != nil {
		s.logger.Error(err)
		return err
	}
	return nil
}

func (s *storage) GetChequeTxsIterator() ChequeTxsIterator {
	return &chequeTxsIterator{Iterator: s.chequeTxsDB.NewIterator(), logger: s.logger}
}

type ChequeTxsIterator interface {
	Next() bool
	Error() error
	Value() (txID ids.ID, cheque *models.SignedCheque, err error)
	Release()
}

var _ ChequeTxsIterator = (*chequeTxsIterator)(nil)

type chequeTxsIterator struct {
	logger *zap.SugaredLogger
	database.Iterator
}

func (it *chequeTxsIterator) Value() (txID ids.ID, cheque *models.SignedCheque, err error) {
	key := it.Iterator.Key()
	txID, err = ids.ToID(key)
	if err != nil {
		it.logger.Error(err)
		return ids.Empty, nil, err
	}
	cheque = &models.SignedCheque{}
	if _, err := codec.Unmarshal(it.Iterator.Value(), cheque); err != nil {
		it.logger.Error(err)
		return ids.Empty, nil, err
	}
	return txID, cheque, nil
}
