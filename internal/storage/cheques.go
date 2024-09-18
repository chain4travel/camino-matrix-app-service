package storage

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"

	"github.com/chain4travel/camino-messenger-bot/pkg/cheques"
	"github.com/chain4travel/camino-synapse-app-service/internal/models"
	"github.com/ethereum/go-ethereum/common"
)

const chequebooksTableName = "chequebooks"

var (
	_ ChequesStorage = (*session)(nil)

	zeroHash = common.Hash{}
)

type ChequesStorage interface {
	GetChequebook(ctx context.Context, chequebookID common.Hash) (*models.Chequebook, error)
	AddCheque(ctx context.Context, chequebookID common.Hash, cheque *cheques.SignedCheque) error // TODO@ accept chequebook, not cheque
	UpdateChequebook(ctx context.Context, chequebook *models.Chequebook) error
	GetNotCashedChequebooks(ctx context.Context) ([]models.Chequebook, error)
}

type chequebook struct {
	ChequebookID  common.Hash            `db:"chequebook_id"`
	FromCMAccount common.Address         `db:"from_cm_account"`
	ToCMAccount   common.Address         `db:"to_cm_account"`
	ToBot         common.Address         `db:"to_bot"`
	Counter       *big.Int               `db:"counter"`
	Amount        *big.Int               `db:"amount"`
	CreatedAt     *big.Int               `db:"created_at"`
	ExpiresAt     *big.Int               `db:"expires_at"`
	Signature     []byte                 `db:"signature"`
	TxID          *string                `db:"tx_id"`
	Status        *models.ChequeTxStatus `db:"status"`
}

func (s *session) GetChequebook(ctx context.Context, chequebookID common.Hash) (*models.Chequebook, error) {
	chequebook := &chequebook{}
	if err := s.tx.StmtxContext(ctx, s.storage.getChequeByID).GetContext(ctx, chequebook, chequebookID); err != nil {
		if err != sql.ErrNoRows {
			s.logger.Error(err)
		}
		return nil, upgradeError(err)
	}
	return modelFromChequebook(chequebook)
}

func (s *session) AddCheque(ctx context.Context, chequebookID common.Hash, cheque *cheques.SignedCheque) error {
	result, err := s.tx.NamedStmtContext(ctx, s.storage.addChequebook).
		ExecContext(ctx, chequebookFromCheque(chequebookID, cheque))
	if err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	}
	if rowsAffected, err := result.RowsAffected(); err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	} else if rowsAffected != 1 {
		return fmt.Errorf("failed to add cheque: expected to affect 1 row, but affected %d", rowsAffected)
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
		return fmt.Errorf("failed to update cheque: expected to affect 1 row, but affected %d", rowsAffected)
	}
	return nil
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
			s.logger.Errorf("failed to get not cashed cheque from db: %v", err)
			continue
		}
		model, err := modelFromChequebook(chequebook)
		if err != nil {
			s.logger.Errorf("failed to parse not cashed cheque: %v", err)
			continue
		}
		chequebooks = append(chequebooks, *model)
	}
	return chequebooks, nil
}

func (s *storage) prepareChequesStmts(ctx context.Context) error {
	getChequeByID, err := s.db.PreparexContext(ctx, fmt.Sprintf(`
		SELECT * FROM %s
		WHERE chequebook_id = ?
	`, chequebooksTableName))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.getChequeByID = getChequeByID

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

	getNotCashedChequebooks, err := s.db.PreparexContext(ctx, fmt.Sprintf(`
		SELECT * FROM %s
		WHERE status != %d OR status IS NULL
	`, chequebooksTableName, models.ChequeTxStatusAccepted))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.getNotCashedChequebooks = getNotCashedChequebooks

	return nil
}

func modelFromChequebook(chequebook *chequebook) (*models.Chequebook, error) {
	txID := common.Hash{}
	if chequebook.TxID != nil {
		txID = common.HexToHash(*chequebook.TxID)
	}

	status := models.ChequeTxStatusUnknown
	if chequebook.Status != nil {
		status = models.ChequeTxStatus(*chequebook.Status)
	}

	return &models.Chequebook{
		SignedCheque: cheques.SignedCheque{
			Cheque: cheques.Cheque{
				FromCMAccount: chequebook.FromCMAccount,
				ToCMAccount:   chequebook.ToCMAccount,
				ToBot:         chequebook.ToBot,
				Counter:       chequebook.Counter,
				Amount:        chequebook.Amount,
				CreatedAt:     chequebook.CreatedAt,
				ExpiresAt:     chequebook.ExpiresAt,
			},
			Signature: chequebook.Signature,
		},
		ChequebookID: chequebook.ChequebookID,
		TxID:         txID,
		Status:       status,
	}, nil
}

func chequebookFromModel(model *models.Chequebook) *chequebook {
	var txID *string
	if model.TxID != zeroHash {
		parsedTxID := model.TxID.Hex()
		txID = &parsedTxID
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
		Counter:       model.Counter,
		Amount:        model.Amount,
		CreatedAt:     model.CreatedAt,
		ExpiresAt:     model.ExpiresAt,
		Signature:     model.Signature,
		TxID:          txID,
		Status:        status,
	}
}

func chequebookFromCheque(chequebookID common.Hash, cheque *cheques.SignedCheque) *chequebook {
	return &chequebook{
		ChequebookID:  chequebookID,
		FromCMAccount: cheque.FromCMAccount,
		ToCMAccount:   cheque.ToCMAccount,
		ToBot:         cheque.ToBot,
		Counter:       cheque.Counter,
		Amount:        cheque.Amount,
		CreatedAt:     cheque.CreatedAt,
		ExpiresAt:     cheque.ExpiresAt,
		Signature:     cheque.Signature,
	}
}
