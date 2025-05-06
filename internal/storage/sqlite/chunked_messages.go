// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/chain4travel/camino-matrix-app-service/internal/service"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jmoiron/sqlx"
)

const chunkedMessagesTableName = "chunked_messages"
const chequeRecordsTableName = "cheque_records"

var (
	_ service.Storage = (*storage)(nil)

	zeroHash = common.Hash{}
)

type chunkedMessage struct {
	MessageID            string `db:"message_id"`
	StoredChunksNumber   uint64 `db:"stored_chunks_number"`
	ExpectedChunksNumber uint64 `db:"expected_chunks_number"`
}

func (s *storage) GetChunksNumbers(ctx context.Context, session service.Session, messageID string) (uint64, uint64, error) {
	tx, err := getSQLXTx(session)
	if err != nil {
		s.base.Logger.Error(err)
		return 0, 0, err
	}

	chunkedMessage := &chunkedMessage{}
	if err := tx.StmtxContext(ctx, s.getChunkNumbers).GetContext(ctx, chunkedMessage, messageID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			s.base.Logger.Error(err)
		}
		return 0, 0, upgradeError(err)
	}
	return chunkedMessage.StoredChunksNumber, chunkedMessage.ExpectedChunksNumber, nil
}

func (s *storage) InsertChunkedMessage(ctx context.Context, session service.Session, messageID string, chunksNumber uint64) error {
	tx, err := getSQLXTx(session)
	if err != nil {
		s.base.Logger.Error(err)
		return err
	}

	result, err := tx.NamedStmtContext(ctx, s.insertChunkNumbers).ExecContext(ctx, chunkedMessage{
		MessageID:            messageID,
		StoredChunksNumber:   1,
		ExpectedChunksNumber: chunksNumber,
	})
	if err != nil {
		s.base.Logger.Error(err)
		return upgradeError(err)
	}
	if rowsAffected, err := result.RowsAffected(); err != nil {
		s.base.Logger.Error(err)
		return upgradeError(err)
	} else if rowsAffected != 1 {
		return fmt.Errorf("error while adding chunked message: expected to affect 1 row, but affected %d", rowsAffected)
	}
	return nil
}

func (s *storage) AddMessageChunk(ctx context.Context, session service.Session, messageID string) error {
	tx, err := getSQLXTx(session)
	if err != nil {
		s.base.Logger.Error(err)
		return err
	}

	result, err := tx.StmtxContext(ctx, s.addMessageChunk).ExecContext(ctx, messageID)
	if err != nil {
		s.base.Logger.Error(err)
		return upgradeError(err)
	}
	if rowsAffected, err := result.RowsAffected(); err != nil {
		s.base.Logger.Error(err)
		return upgradeError(err)
	} else if rowsAffected != 1 {
		return fmt.Errorf("error while adding message chunk: expected to affect 1 row, but affected %d", rowsAffected)
	}
	return nil
}

func (s *storage) DeleteChunkedMessage(ctx context.Context, session service.Session, messageID string) error {
	tx, err := getSQLXTx(session)
	if err != nil {
		s.base.Logger.Error(err)
		return err
	}

	result, err := tx.StmtxContext(ctx, s.deleteChunkedMessage).ExecContext(ctx, messageID)
	if err != nil {
		s.base.Logger.Error(err)
		return upgradeError(err)
	}
	if rowsAffected, err := result.RowsAffected(); err != nil {
		s.base.Logger.Error(err)
		return upgradeError(err)
	} else if rowsAffected != 1 {
		return fmt.Errorf("error while deleting chunked message: expected to affect 1 row, but affected %d", rowsAffected)
	}
	return nil
}

type chunkedMessagesStatements struct {
	getChunkNumbers, addMessageChunk, deleteChunkedMessage *sqlx.Stmt
	insertChunkNumbers                                     *sqlx.NamedStmt
}

func (s *storage) prepareChunkedMessagesStmts(ctx context.Context) error {
	getChunkNumbers, err := s.base.DB.PreparexContext(ctx, fmt.Sprintf(`
		SELECT stored_chunks_number, expected_chunks_number FROM %s
		WHERE message_id = ?
	`, chunkedMessagesTableName))
	if err != nil {
		s.base.Logger.Error(err)
		return err
	}
	s.getChunkNumbers = getChunkNumbers

	insertChunkNumbers, err := s.base.DB.PrepareNamedContext(ctx, fmt.Sprintf(`
		INSERT INTO %s (
			message_id,
			stored_chunks_number,
			expected_chunks_number
		) VALUES (
			:message_id,
			:stored_chunks_number,
			:expected_chunks_number
		)
	`, chunkedMessagesTableName))
	if err != nil {
		s.base.Logger.Error(err)
		return err
	}
	s.insertChunkNumbers = insertChunkNumbers

	addMessageChunk, err := s.base.DB.PreparexContext(ctx, fmt.Sprintf(`
		UPDATE %s
		SET stored_chunks_number = stored_chunks_number + 1
		WHERE message_id = ?
	`, chunkedMessagesTableName))
	if err != nil {
		s.base.Logger.Error(err)
		return err
	}
	s.addMessageChunk = addMessageChunk

	deleteChunkedMessage, err := s.base.DB.PreparexContext(ctx, fmt.Sprintf(`
		DELETE FROM %s
		WHERE message_id = ?
	`, chunkedMessagesTableName))
	if err != nil {
		s.base.Logger.Error(err)
		return err
	}
	s.deleteChunkedMessage = deleteChunkedMessage

	return nil
}

type chequeRecordsStatements struct {
	getNotCashedChequeRecords, getChequeRecordsWithPendingTxs *sqlx.Stmt
	getChequeRecordByID, getChequeRecordByTxID                *sqlx.Stmt
	upsertChequeRecord                                        *sqlx.NamedStmt
}

// func (s *storage) prepareChequeRecordsStmts(ctx context.Context) error {

// 	upsertChequeRecord, err := s.base.DB.PrepareNamedContext(ctx, fmt.Sprintf(`
// 		INSERT INTO %s (
// 			cheque_record_id,
// 			from_cm_account,
// 			to_cm_account,
// 			to_bot,
// 			counter,
// 			amount,
// 			created_at,
// 			expires_at,
// 			signature,
// 			tx_id,
// 			status
// 		) VALUES (
// 			:cheque_record_id,
// 			:from_cm_account,
// 			:to_cm_account,
// 			:to_bot,
// 			:counter,
// 			:amount,
// 			:created_at,
// 			:expires_at,
// 			:signature,
// 			:tx_id,
// 			:status
// 		)
// 		ON CONFLICT(cheque_record_id)
// 		DO UPDATE SET
// 			counter     = excluded.counter,
// 			amount      = excluded.amount,
// 			created_at  = excluded.created_at,
// 			expires_at  = excluded.expires_at,
// 			signature   = excluded.signature,
// 			tx_id       = excluded.tx_id,
// 			status      = excluded.status
// 	`, chequeRecordsTableName))
// 	if err != nil {
// 		s.base.Logger.Error(err)
// 		return err
// 	}
// 	s.upsertChequeRecord = upsertChequeRecord

// 	return nil
// }

// func modelFromChequeRecord(chequeRecord *chequeRecord) *chequehandler.ChequeRecord {
// 	txID := common.Hash{}
// 	if chequeRecord.TxID != nil {
// 		txID = *chequeRecord.TxID
// 	}

// 	status := chequehandler.ChequeTxStatusUnknown
// 	if chequeRecord.Status != nil {
// 		status = *chequeRecord.Status
// 	}

// 	return &chequehandler.ChequeRecord{
// 		SignedCheque: cheques.SignedCheque{
// 			Cheque: cheques.Cheque{
// 				FromCMAccount: chequeRecord.FromCMAccount,
// 				ToCMAccount:   chequeRecord.ToCMAccount,
// 				ToBot:         chequeRecord.ToBot,
// 				Counter:       big.NewInt(0).SetBytes(chequeRecord.Counter),
// 				Amount:        big.NewInt(0).SetBytes(chequeRecord.Amount),
// 				CreatedAt:     big.NewInt(0).SetBytes(chequeRecord.CreatedAt),
// 				ExpiresAt:     big.NewInt(0).SetBytes(chequeRecord.ExpiresAt),
// 			},
// 			Signature: chequeRecord.Signature,
// 		},
// 		ChequeRecordID: chequeRecord.ChequeRecordID,
// 		TxID:           txID,
// 		Status:         status,
// 	}
// }

// func chequeRecordFromModel(model *chequehandler.ChequeRecord) *chequeRecord {
// 	var txID *common.Hash
// 	if model.TxID != zeroHash {
// 		txID = &model.TxID
// 	}

// 	var status *chequehandler.ChequeTxStatus
// 	if model.Status != chequehandler.ChequeTxStatusUnknown {
// 		status = &model.Status
// 	}

// 	return &chequeRecord{
// 		ChequeRecordID: model.ChequeRecordID,
// 		FromCMAccount:  model.FromCMAccount,
// 		ToCMAccount:    model.ToCMAccount,
// 		ToBot:          model.ToBot,
// 		Counter:        model.Counter.Bytes(),
// 		Amount:         model.Amount.Bytes(),
// 		CreatedAt:      model.CreatedAt.Bytes(),
// 		ExpiresAt:      model.ExpiresAt.Bytes(),
// 		Signature:      model.Signature,
// 		TxID:           txID,
// 		Status:         status,
// 	}
// }
