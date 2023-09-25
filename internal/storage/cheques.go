package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	tStatus "github.com/ava-labs/avalanchego/vms/touristicvm/status"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
	"github.com/chain4travel/camino-synapse-app-service/internal/models"
)

const chequesTableName = "cheques"

type cheque struct {
	ChequebookID string  `db:"chequebook_id"`
	Issuer       string  `db:"issuer"`
	Agent        string  `db:"agent"`
	Beneficiary  string  `db:"beneficiary"`
	Amount       uint64  `db:"amount"`
	SerialID     uint64  `db:"serial_id"`
	Signature    []byte  `db:"signature"`
	TxID         *string `db:"tx_id"`
	Status       *uint32 `db:"status"`
}

func (s *session) GetCheque(ctx context.Context, chequebookID string) (*models.Cheque, error) {
	cheque := &cheque{}
	if err := s.tx.StmtxContext(ctx, s.storage.getChequeByID).GetContext(ctx, cheque, chequebookID); err != nil {
		if err != sql.ErrNoRows {
			s.logger.Error(err)
		}
		return nil, upgradeError(err)
	}
	return s.storage.modelFromCheque(cheque)
}

func (s *session) AddCheque(ctx context.Context, cheque *models.Cheque) error {
	result, err := s.tx.NamedStmtContext(ctx, s.storage.addCheque).
		ExecContext(ctx, chequeFromModel(cheque))
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

func (s *session) UpdateCheque(ctx context.Context, cheque *models.Cheque) error {
	result, err := s.tx.NamedStmtContext(ctx, s.storage.updateCheque).
		ExecContext(ctx, chequeFromModel(cheque))
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

func (s *session) GetNotCashedCheques(ctx context.Context) ([]models.Cheque, error) {
	cheques := []models.Cheque{}
	rows, err := s.tx.StmtxContext(ctx, s.storage.getNotCashedCheques).QueryxContext(ctx)
	if err != nil {
		s.logger.Error(err)
		return nil, upgradeError(err)
	}
	for rows.Next() {
		cheque := &cheque{}
		if err := rows.StructScan(cheque); err != nil {
			s.logger.Error(err)
			return nil, upgradeError(err)
		}
		model, err := s.storage.modelFromCheque(cheque)
		if err != nil {
			return nil, err
		}
		cheques = append(cheques, *model)
	}
	return cheques, nil
}

func (s *storage) prepareChequesStmts(ctx context.Context) error {
	getChequeByID, err := s.db.PreparexContext(ctx, fmt.Sprintf(`
		SELECT * FROM %s
		WHERE chequebook_id = ?
	`, chequesTableName))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.getChequeByID = getChequeByID

	addCheque, err := s.db.PrepareNamedContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (
			chequebook_id,
			issuer,
			agent,
			beneficiary,
			amount,
			serial_id,
			signature
		) VALUES (
			:chequebook_id,
			:issuer,
			:agent,
			:beneficiary,
			:amount,
			:serial_id,
			:signature
		)
	`, chequesTableName))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.addCheque = addCheque

	updateCheque, err := s.db.PrepareNamedContext(ctx, fmt.Sprintf(`
		UPDATE %s
		SET amount      = :amount,
			serial_id   = :serial_id,
			signature   = :signature,
			tx_id       = :tx_id,
			status      = :status
		WHERE chequebook_id = :chequebook_id
	`, chequesTableName))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.updateCheque = updateCheque

	getNotCashedCheques, err := s.db.PreparexContext(ctx, fmt.Sprintf(`
		SELECT * FROM %s
		WHERE status != %d OR status IS NULL
	`, chequesTableName, tStatus.Committed))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.getNotCashedCheques = getNotCashedCheques

	return nil
}

func (s *storage) modelFromCheque(cheque *cheque) (*models.Cheque, error) {
	issuerID, err := ids.ShortFromString(cheque.Issuer)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}

	agentID, err := ids.ShortFromString(cheque.Agent)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}

	beneficiaryID, err := ids.ShortFromString(cheque.Beneficiary)
	if err != nil {
		s.logger.Error(err)
		return nil, err
	}

	txID := ids.Empty
	if cheque.TxID != nil {
		txID, err = ids.FromString(*cheque.TxID)
		if err != nil {
			s.logger.Error(err)
			return nil, err
		}
	}

	txStatus := tStatus.Unknown
	if cheque.Status != nil {
		txStatus = tStatus.Status(*cheque.Status)
	}

	signature := [secp256k1.SignatureLen]byte{}
	copy(signature[:], cheque.Signature)

	return &models.Cheque{
		ChequebookID: cheque.ChequebookID,
		Cheque: txs.Cheque{
			Issuer:      issuerID,
			Agent:       agentID,
			Beneficiary: beneficiaryID,
			Amount:      cheque.Amount,
			SerialID:    cheque.SerialID,
		},
		Signature: signature,
		TxID:      txID,
		Status:    txStatus,
	}, nil
}

func chequeFromModel(model *models.Cheque) *cheque {
	txID := model.TxID.String()
	txStatus := uint32(model.Status)
	return &cheque{
		ChequebookID: model.ChequebookID,
		Issuer:       model.Issuer.String(),
		Agent:        model.Agent.String(),
		Beneficiary:  model.Beneficiary.String(),
		Amount:       model.Amount,
		SerialID:     model.SerialID,
		Signature:    model.Signature[:],
		TxID:         &txID,
		Status:       &txStatus,
	}
}
