package storage

import (
	"camino-synapse-appservice/internal/models"
	"context"
	"sync"

	"github.com/ava-labs/avalanchego/database"
	"github.com/ava-labs/avalanchego/database/prefixdb"
	"github.com/ava-labs/avalanchego/database/versiondb"
	"github.com/ava-labs/avalanchego/ids"
	"go.uber.org/zap"
)

const (
	chequeRecordsPrefix = "chequeRecords"
	chequeTxsPrefix     = "chequeTxs"
)

var _ Storage = (*storage)(nil)

type Storage interface {
	NewSession()
	Commit() error
	Abort()
	Close(ctx context.Context) error

	GetTrio(trioID string) (*models.Trio, error)
	SetTrio(trioID string, record *models.Trio) error
	GetTriosIterator() ChequeTriosIterator

	RemoveChequeTx(txID ids.ID) error
	AddChequeTx(txID ids.ID, cheque *models.SignedCheque) error
	GetChequeTxsIterator() ChequeTxsIterator
}

func New(ctx context.Context, logger *zap.SugaredLogger, path string) (Storage, error) {
	db, err := newDatabase(path)
	if err != nil {
		logger.Error(err)
		return nil, err
	}
	rootDB := versiondb.New(db)
	return &storage{
		logger:      logger,
		db:          db,
		rootDB:      rootDB,
		chequesDB:   prefixdb.New([]byte(chequeRecordsPrefix), rootDB),
		chequeTxsDB: prefixdb.New([]byte(chequeTxsPrefix), rootDB),
	}, nil
}

type storage struct {
	logger      *zap.SugaredLogger
	db          database.Database
	rootDB      *versiondb.Database
	chequesDB   database.Database
	chequeTxsDB database.Database

	comitted bool
	lock     sync.Mutex
}

func (s *storage) NewSession() {
	// cause leveldb batch doesn't lock data, data could become inconsistent, so we do lock
	s.lock.Lock()
}

func (s *storage) Commit() error {
	if err := s.rootDB.Commit(); err != nil {
		s.logger.Error(err)
		s.rootDB.Abort()
		return err
	}
	s.comitted = true
	s.lock.Unlock()
	return nil
}

func (s *storage) Abort() {
	s.rootDB.Abort()
	if s.comitted {
		return
	}
	s.lock.Unlock()
}

func (s *storage) Close(ctx context.Context) error {
	if err := s.db.Close(); err != nil {
		s.logger.Error(err)
		return err
	}
	if err := s.rootDB.Close(); err != nil {
		s.logger.Error(err)
		return err
	}
	if err := s.chequesDB.Close(); err != nil {
		s.logger.Error(err)
		return err
	}
	if err := s.chequeTxsDB.Close(); err != nil {
		s.logger.Error(err)
		return err
	}
	return nil
}
