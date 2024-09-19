package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math/big"

	"github.com/chain4travel/camino-messenger-bot/pkg/cheques"
	"github.com/chain4travel/camino-synapse-app-service/internal/models"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jmoiron/sqlx"
)

const chequebooksTableName = "chequebooks"

var (
	_ ChequesStorage = (*session)(nil)

	zeroHash = common.Hash{}
)

type ChequesStorage interface {
	GetNotCashedChequebooks(ctx context.Context) ([]models.Chequebook, error)
	GetChequebooksWithPendingTxs(ctx context.Context) ([]models.Chequebook, error)
	GetChequebook(ctx context.Context, chequebookID common.Hash) (*models.Chequebook, error)
	GetChequebookByTxID(ctx context.Context, txID common.Hash) (*models.Chequebook, error)
	AddChequebook(ctx context.Context, chequebook *models.Chequebook) error
	UpdateChequebook(ctx context.Context, chequebook *models.Chequebook) error
}

type chequebook struct {
	ChequebookID  common.Hash            `db:"chequebook_id"`
	FromCMAccount common.Address         `db:"from_cm_account"`
	ToCMAccount   common.Address         `db:"to_cm_account"`
	ToBot         common.Address         `db:"to_bot"`
	Counter       []byte                 `db:"counter"`
	Amount        []byte                 `db:"amount"`
	CreatedAt     []byte                 `db:"created_at"`
	ExpiresAt     []byte                 `db:"expires_at"`
	Signature     []byte                 `db:"signature"`
	TxID          *common.Hash           `db:"tx_id"`
	Status        *models.ChequeTxStatus `db:"status"`
}

func (s *session) GetNotCashedChequebooks(ctx context.Context) ([]models.Chequebook, error) {
	chequebooks := []models.Chequebook{}
	rows, err := s.tx.StmtxContext(ctx, s.storage.getNotCashedChequebooks).QueryxContext(ctx)
	if err != nil {
		s.logger.Error(err)
		return nil, upgradeError(err)
	}
	for rows.Next() {
		chequebook := &chequebook{}
		if err := rows.StructScan(chequebook); err != nil {
			s.logger.Errorf("failed to get not cashed chequebook from db: %v", err)
			continue
		}
		model, err := modelFromChequebook(chequebook)
		if err != nil {
			s.logger.Errorf("failed to parse not cashed chequebook: %v", err)
			continue
		}
		chequebooks = append(chequebooks, *model)
	}
	return chequebooks, nil
}

func (s *session) GetChequebooksWithPendingTxs(ctx context.Context) ([]models.Chequebook, error) {
	chequebooks := []models.Chequebook{}
	rows, err := s.tx.StmtxContext(ctx, s.storage.getChequebooksWithPendingTxs).QueryxContext(ctx)
	if err != nil {
		s.logger.Error(err)
		return nil, upgradeError(err)
	}
	for rows.Next() {
		chequebook := &chequebook{}
		if err := rows.StructScan(chequebook); err != nil {
			s.logger.Errorf("failed to get chequebook with pending tx from db: %v", err)
			continue
		}
		model, err := modelFromChequebook(chequebook)
		if err != nil {
			s.logger.Errorf("failed to parse chequebook with pending tx: %v", err)
			continue
		}
		chequebooks = append(chequebooks, *model)
	}
	return chequebooks, nil
}

func (s *session) GetChequebook(ctx context.Context, chequebookID common.Hash) (*models.Chequebook, error) {
	chequebook := &chequebook{}
	if err := s.tx.StmtxContext(ctx, s.storage.getChequeByChequebookID).GetContext(ctx, chequebook, chequebookID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Error(err)
		}
		return nil, upgradeError(err)
	}
	return modelFromChequebook(chequebook)
}

func (s *session) GetChequebookByTxID(ctx context.Context, txID common.Hash) (*models.Chequebook, error) {
	chequebook := &chequebook{}
	if err := s.tx.StmtxContext(ctx, s.storage.getChequeByTxID).GetContext(ctx, chequebook, txID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Error(err)
		}
		return nil, upgradeError(err)
	}
	return modelFromChequebook(chequebook)
}

func (s *session) AddChequebook(ctx context.Context, chequebook *models.Chequebook) error {
	result, err := s.tx.NamedStmtContext(ctx, s.storage.addChequebook).
		ExecContext(ctx, chequebookFromModel(chequebook))
	if err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	}
	if rowsAffected, err := result.RowsAffected(); err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	} else if rowsAffected != 1 {
		return fmt.Errorf("failed to add chequebook: expected to affect 1 row, but affected %d", rowsAffected)
	}
	return nil
}

func (s *session) UpdateChequebook(ctx context.Context, chequebook *models.Chequebook) error {
	result, err := s.tx.NamedStmtContext(ctx, s.storage.updateChequebook).
		ExecContext(ctx, chequebookFromModel(chequebook))
	if err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	}
	if rowsAffected, err := result.RowsAffected(); err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	} else if rowsAffected != 1 {
		return fmt.Errorf("failed to update chequebook: expected to affect 1 row, but affected %d", rowsAffected)
	}
	return nil
}

type chequebooksStatements struct {
	getNotCashedChequebooks, getChequebooksWithPendingTxs *sqlx.Stmt
	getChequeByChequebookID, getChequeByTxID              *sqlx.Stmt
	addChequebook, updateChequebook                       *sqlx.NamedStmt
}

func (s *storage) prepareChequebooksStmts(ctx context.Context) error {
	getNotCashedChequebooks, err := s.db.PreparexContext(ctx, fmt.Sprintf(`
		SELECT * FROM %s
		WHERE status = %d OR status IS NULL
	`, chequebooksTableName, models.ChequeTxStatusRejected))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.getNotCashedChequebooks = getNotCashedChequebooks

	getChequebooksWithPendingTxs, err := s.db.PreparexContext(ctx, fmt.Sprintf(`
		SELECT * FROM %s
		WHERE status = %d
	`, chequebooksTableName, models.ChequeTxStatusProcessing))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.getChequebooksWithPendingTxs = getChequebooksWithPendingTxs

	getChequeByID, err := s.db.PreparexContext(ctx, fmt.Sprintf(`
		SELECT * FROM %s
		WHERE chequebook_id = ?
	`, chequebooksTableName))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.getChequeByChequebookID = getChequeByID

	getChequeByTxID, err := s.db.PreparexContext(ctx, fmt.Sprintf(`
		SELECT * FROM %s
		WHERE tx_id = ?
	`, chequebooksTableName))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.getChequeByTxID = getChequeByTxID

	addChequebook, err := s.db.PrepareNamedContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (
			chequebook_id,
			from_cm_account,
			to_cm_account,
			to_bot,
			counter,
			amount,
			created_at,
			expires_at,
			signature
		) VALUES (
			:chequebook_id,
			:from_cm_account,
			:to_cm_account,
			:to_bot,
			:counter,
			:amount,
			:created_at,
			:expires_at,
			:signature
		)
	`, chequebooksTableName))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.addChequebook = addChequebook

	updateChequebook, err := s.db.PrepareNamedContext(ctx, fmt.Sprintf(`
		UPDATE %s
		SET counter    = :counter,
			amount     = :amount,
			created_at = :created_at,
			expires_at = :expires_at,
			signature  = :signature,
			tx_id      = :tx_id,
			status     = :status
		WHERE chequebook_id = :chequebook_id
	`, chequebooksTableName))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.updateChequebook = updateChequebook

	return nil
}

func modelFromChequebook(chequebook *chequebook) (*models.Chequebook, error) {
	txID := common.Hash{}
	if chequebook.TxID != nil {
		txID = *chequebook.TxID
	}

	status := models.ChequeTxStatusUnknown
	if chequebook.Status != nil {
		status = *chequebook.Status
	}

	return &models.Chequebook{
		SignedCheque: cheques.SignedCheque{
			Cheque: cheques.Cheque{
				FromCMAccount: chequebook.FromCMAccount,
				ToCMAccount:   chequebook.ToCMAccount,
				ToBot:         chequebook.ToBot,
				Counter:       big.NewInt(0).SetBytes(chequebook.Counter),
				Amount:        big.NewInt(0).SetBytes(chequebook.Amount),
				CreatedAt:     big.NewInt(0).SetBytes(chequebook.CreatedAt),
				ExpiresAt:     big.NewInt(0).SetBytes(chequebook.ExpiresAt),
			},
			Signature: chequebook.Signature,
		},
		ChequebookID: chequebook.ChequebookID,
		TxID:         txID,
		Status:       status,
	}, nil
}

func chequebookFromModel(model *models.Chequebook) *chequebook {
	var txID *common.Hash
	if model.TxID != zeroHash {
		txID = &model.TxID
	}

	var status *models.ChequeTxStatus
	if model.Status != models.ChequeTxStatusUnknown {
		status = &model.Status
	}

	return &chequebook{
		ChequebookID:  model.ChequebookID,
		FromCMAccount: model.FromCMAccount,
		ToCMAccount:   model.ToCMAccount,
		ToBot:         model.ToBot,
		Counter:       model.Counter.Bytes(),
		Amount:        model.Amount.Bytes(),
		CreatedAt:     model.CreatedAt.Bytes(),
		ExpiresAt:     model.ExpiresAt.Bytes(),
		Signature:     model.Signature,
		TxID:          txID,
		Status:        status,
	}
}
