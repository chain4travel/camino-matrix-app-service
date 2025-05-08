// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package service

import (
	"context"
)

type Storage interface {
	SessionHandler
	MessageChunksStorage
}

type MessageChunksStorage interface {
	GetChunksNumbers(ctx context.Context, session Session, messageID string) (storedChunksNumber uint64, expectedChunksNumber uint64, err error)
	InsertChunkedMessage(ctx context.Context, session Session, messageID string, chunksNumber uint64) error
	AddMessageChunk(ctx context.Context, session Session, messageID string) error
	DeleteChunkedMessage(ctx context.Context, session Session, messageID string) error
}

type SessionHandler interface {
	NewSession(ctx context.Context) (Session, error)
	Commit(session Session) error
	Abort(session Session)
}

type Session interface {
	Commit() error
	Abort() error
}
