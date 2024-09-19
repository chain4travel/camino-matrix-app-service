package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/jmoiron/sqlx"
)

const chunkedMessagesTableName = "chunked_messages"

var _ ChunkedMessagesStorage = (*session)(nil)

type ChunkedMessagesStorage interface {
	GetChunksNumbers(ctx context.Context, messageID string) (storedChunksNumber uint64, expectedChunksNumber uint64, err error)
	AddChunkedMessage(ctx context.Context, messageID string, chunksNumber uint64) error
	AddMessageChunk(ctx context.Context, messageID string) error
	DeleteChunkedMessage(ctx context.Context, messageID string) error
}

type chunkedMessage struct {
	MessageID            string `db:"message_id"`
	StoredChunksNumber   uint64 `db:"stored_chunks_number"`
	ExpectedChunksNumber uint64 `db:"expected_chunks_number"`
}

func (s *session) GetChunksNumbers(ctx context.Context, messageID string) (uint64, uint64, error) {
	chunkedMessage := &chunkedMessage{}
	if err := s.tx.StmtxContext(ctx, s.storage.getChunkNumbers).GetContext(ctx, chunkedMessage, messageID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			s.logger.Error(err)
		}
		return 0, 0, upgradeErrorAllowNotFound(err)
	}
	return chunkedMessage.StoredChunksNumber, chunkedMessage.ExpectedChunksNumber, nil
}

func (s *session) AddChunkedMessage(ctx context.Context, messageID string, chunksNumber uint64) error {
	result, err := s.tx.NamedStmtContext(ctx, s.storage.addChunkNumbers).
		ExecContext(ctx, chunkedMessage{
			MessageID:            messageID,
			StoredChunksNumber:   1,
			ExpectedChunksNumber: chunksNumber,
		})
	if err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	}
	if rowsAffected, err := result.RowsAffected(); err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	} else if rowsAffected != 1 {
		return fmt.Errorf("error while adding chunked message: expected to affect 1 row, but affected %d", rowsAffected)
	}
	return nil
}

func (s *session) AddMessageChunk(ctx context.Context, messageID string) error {
	result, err := s.tx.StmtxContext(ctx, s.storage.addMessageChunk).ExecContext(ctx, messageID)
	if err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	}
	if rowsAffected, err := result.RowsAffected(); err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	} else if rowsAffected != 1 {
		return fmt.Errorf("error while adding message chunk: expected to affect 1 row, but affected %d", rowsAffected)
	}
	return nil
}

func (s *session) DeleteChunkedMessage(ctx context.Context, messageID string) error {
	result, err := s.tx.StmtxContext(ctx, s.storage.deleteChunkedMessage).ExecContext(ctx, messageID)
	if err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	}
	if rowsAffected, err := result.RowsAffected(); err != nil {
		s.logger.Error(err)
		return upgradeError(err)
	} else if rowsAffected != 1 {
		return fmt.Errorf("error while deleting chunked message: expected to affect 1 row, but affected %d", rowsAffected)
	}
	return nil
}

type chunkedMessagesStatements struct {
	getChunkNumbers, addMessageChunk, deleteChunkedMessage *sqlx.Stmt
	addChunkNumbers                                        *sqlx.NamedStmt
}

func (s *storage) prepareChunkedMessagesStmts(ctx context.Context) error {
	getChunkNumbers, err := s.db.PreparexContext(ctx, fmt.Sprintf(`
		SELECT stored_chunks_number, expected_chunks_number FROM %s
		WHERE message_id = ?
	`, chunkedMessagesTableName))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.getChunkNumbers = getChunkNumbers

	addChunkNumbers, err := s.db.PrepareNamedContext(ctx, fmt.Sprintf(`
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
		s.logger.Error(err)
		return err
	}
	s.addChunkNumbers = addChunkNumbers

	addMessageChunk, err := s.db.PreparexContext(ctx, fmt.Sprintf(`
		UPDATE %s
		SET stored_chunks_number = stored_chunks_number + 1
		WHERE message_id = ?
	`, chunkedMessagesTableName))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.addMessageChunk = addMessageChunk

	deleteChunkedMessage, err := s.db.PreparexContext(ctx, fmt.Sprintf(`
		DELETE FROM %s
		WHERE message_id = ?
	`, chunkedMessagesTableName))
	if err != nil {
		s.logger.Error(err)
		return err
	}
	s.deleteChunkedMessage = deleteChunkedMessage

	return nil
}
