// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/chain4travel/camino-matrix-app-service/internal/service"
	"github.com/jmoiron/sqlx"
)

const chunkedMessagesTableName = "chunked_messages"

var _ service.MessageChunksStorage = (*storage)(nil)

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
