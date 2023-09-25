package storage

import (
	"context"
	"database/sql"
	"errors"

	"github.com/chain4travel/camino-synapse-app-service/internal/models"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite3"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"go.uber.org/zap"
)

var (
	_ Storage = (*storage)(nil)

	ErrNotFound = errors.New("not found")
)

type Storage interface {
	NewSession(ctx context.Context) (Session, error)
	Close(ctx context.Context) error
}

type Session interface {
	Commit() error
	Abort()

	GetCheque(ctx context.Context, chequebookID string) (*models.Cheque, error)
	AddCheque(ctx context.Context, cheque *models.Cheque) error
	UpdateCheque(ctx context.Context, cheque *models.Cheque) error
	GetNotCashedCheques(ctx context.Context) ([]models.Cheque, error)
}

func New(ctx context.Context, logger *zap.SugaredLogger, dbPath, dbName, migrationsPath string) (Storage, error) {
	db, err := sqlx.Open("sqlite3", dbPath)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	s := &storage{
		logger: logger,
		db:     db,
	}

	if err := s.migrate(ctx, dbName, migrationsPath); err != nil {
		return nil, err
	}

	if err := s.prepare(ctx); err != nil {
		return nil, err
	}

	return s, nil
}

type storage struct {
	logger *zap.SugaredLogger
	db     *sqlx.DB

	getChequeByID, getNotCashedCheques *sqlx.Stmt
	addCheque, updateCheque            *sqlx.NamedStmt
}

func (s *storage) migrate(ctx context.Context, dbName, migrationsPath string) error {
	s.logger.Infof("Performing db migrations...")

	driver, err := sqlite3.WithInstance(s.db.DB, &sqlite3.Config{})
	if err != nil {
		s.logger.Error(err)
		return err
	}

	migration, err := migrate.NewWithDatabaseInstance(migrationsPath, dbName, driver)
	if err != nil {
		s.logger.Error(err)
		return err
	}

	version, dirty, err := migration.Version()
	if err != nil && err != migrate.ErrNilVersion {
		s.logger.Error(err)
		return err
	}
	if dirty {
		return errors.New("database in dirty state after previous migration, requires manual fixing")
	}
	s.logger.Infof("Migration version: %d", version)

	err = migration.Up()
	switch {
	case err == migrate.ErrNoChange:
		s.logger.Infof("No migrations needed")
	case err != nil:
		s.logger.Error(err)
		return err
	default:
		newVersion, dirty, err := migration.Version()
		if err != nil && err != migrate.ErrNilVersion {
			s.logger.Error(err)
			return err
		}
		if dirty {
			return errors.New("database in dirty state after previous migration, requires manual fixing")
		}
		s.logger.Infof("New migration version: %d", newVersion)
	}

	s.logger.Infof("Finished preforming db migrations")
	return nil
}

func (s *storage) prepare(ctx context.Context) error {
	return s.prepareChequesStmts(ctx)
}

func (s *storage) Close(ctx context.Context) error {
	if err := s.db.Close(); err != nil {
		s.logger.Error(err)
		return err
	}
	return nil
}

func (s *storage) NewSession(ctx context.Context) (Session, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}
	return &session{tx: tx, logger: s.logger, storage: s}, nil
}

type session struct {
	storage  *storage
	logger   *zap.SugaredLogger
	tx       *sqlx.Tx
	commited bool
}

func (s *session) Commit() error {
	if s.commited {
		return errors.New("already commited")
	}
	if err := s.tx.Commit(); err != nil {
		s.logger.Error(err)
		return err
	}
	s.commited = true
	return nil
}

func (s *session) Abort() {
	if s.commited {
		return
	}
	if err := s.tx.Rollback(); err != nil {
		s.logger.Error(err)
	}
}

func upgradeError(err error) error {
	switch err {
	case sql.ErrNoRows:
		return ErrNotFound
	default:
		return err
	}
}
