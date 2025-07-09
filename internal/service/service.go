// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package service

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/chain4travel/camino-matrix-app-service/config"
	"github.com/chain4travel/camino-messenger-bot/v11/pkg/chequehandler"
	cmaccounts "github.com/chain4travel/camino-messenger-bot/v11/pkg/cm_accounts"
	"github.com/chain4travel/camino-messenger-bot/v11/pkg/matrix"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var (
	_ Service = (*service)(nil)

	ErrNotFound = errors.New("not found")
)

type Service interface {
	ProcessEvents(ctx context.Context, events []event.Event) error
}

func NewService(
	logger *zap.SugaredLogger,
	networkFeeRecipientCMAccountAddress common.Address,
	storage Storage,
	ethClient *ethclient.Client,
	chainID *big.Int,
	chequeHandler chequehandler.ChequeHandler,
	cmAccounts cmaccounts.Service,
) Service {
	return &service{
		logger:                              logger,
		ethClient:                           ethClient,
		networkFeeRecipientCMAccountAddress: networkFeeRecipientCMAccountAddress,
		storage:                             storage,
		chainID:                             chainID,
		cmAccounts:                          cmAccounts,
		chequeHandler:                       chequeHandler,
	}
}

type service struct {
	logger                              *zap.SugaredLogger
	ethClient                           *ethclient.Client
	storage                             Storage
	networkFeeRecipientCMAccountAddress common.Address
	chainID                             *big.Int
	cmAccounts                          cmaccounts.Service
	chequeHandler                       chequehandler.ChequeHandler
}

func (s *service) ProcessEvents(ctx context.Context, events []event.Event) error {
	for _, event := range events {
		if event.Type.Type != matrix.EventTypeSignedMessage.Type && event.Type.Type != matrix.EventTypeMessageChunk.Type {
			s.logger.Debugf("Skipping event %s (%s) from %s, not a signed message or message chunk", event.ID, event.Type.Type, event.Sender)
			continue
		}

		banSender, err := s.processMessageEvent(ctx, &event)
		if err != nil {
			return err
		}

		if banSender {
			if err := s.banUser(ctx, event.Sender); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *service) processMessageEvent(ctx context.Context, event *event.Event) (bool, error) {
	s.logger.Debugf("Processing event %s (%s) from %s", event.ID, event.Type.Type, event.Sender)
	defer s.logger.Debugf("Finished processing event %s (%s) from %s", event.ID, event.Type.Type, event.Sender)

	if err := event.Content.ParseRaw(event.Type); err != nil {
		s.logger.Errorf("Failed to parse event content: %v", err)
		// TODO @evlekht ban users for malformed events? e.g. we fail to parse? might be server/lib fault, though it shouldn't just pop up out of nowhere
		return false, err
	}

	switch eventContent := event.Content.Parsed.(type) {
	case *matrix.SignedMessageEventContent:
		return s.processSignedMessageEvent(ctx, eventContent, event.Sender, event.ID)
	case *matrix.MessageChunkEventContent:
		return s.processMessageChunkEvent(ctx, eventContent, event.Sender, event.ID)
	}

	return false, fmt.Errorf("unsupported event type: %s", event.Type.Type)
}

func (s *service) processSignedMessageEvent(ctx context.Context, eventContent *matrix.SignedMessageEventContent, senderBotUserID id.UserID, eventID id.EventID) (bool, error) {
	if err := eventContent.Verify(); err != nil {
		s.logger.Infof("Event %s, message %s from %s: invalid event content: %v", eventID, eventContent.MessageID, senderBotUserID, err)
		return true, err
	}

	session, err := s.storage.NewSession(ctx)
	if err != nil {
		err = fmt.Errorf("failed to create storage session: %w", err)
		s.logger.Errorf("Event %s, message %s: %v", eventID, eventContent.MessageID, err)
		return false, err
	}
	defer s.storage.Abort(session)

	if err := s.chequeHandler.VerifyCheque(
		ctx,
		&eventContent.NetworkFeeCheque,
		matrix.AddressFromUserID(senderBotUserID),
		config.NetworkFee,
	); err != nil {
		s.logger.Infof("Event %s, message %s: failed to verify cheque: %v", eventID, eventContent.MessageID, err)
		return true, nil
	}

	storedChunksCount, _, err := s.storage.GetChunksCount(ctx, session, eventContent.MessageID)
	if err != nil {
		err = fmt.Errorf("failed to get chunks count: %w", err)
		s.logger.Errorf("Event %s, message %s: %v", eventID, eventContent.MessageID, err)
		return false, err
	}

	newChunksCount := storedChunksCount + 1

	if newChunksCount > eventContent.ChunksCount {
		s.logger.Infof("Event %s, message %s: received more chunks than expected (%d > %d)", eventID, eventContent.MessageID, newChunksCount, eventContent.ChunksCount)
		return true, nil
	}

	if newChunksCount == eventContent.ChunksCount {
		if err := s.storage.DeleteChunkedMessage(ctx, session, eventContent.MessageID); err != nil {
			err = fmt.Errorf("failed to delete chunked message: %w", err)
			s.logger.Errorf("Event %s, message %s: %v", eventID, eventContent.MessageID, err)
			return false, err
		}
	} else {
		if err := s.storage.AddFirstChunk(ctx, session, eventContent.MessageID, eventContent.ChunksCount, newChunksCount); err != nil {
			err = fmt.Errorf("failed to add first chunk: %w", err)
			s.logger.Errorf("Event %s, message %s: %v", eventID, eventContent.MessageID, err)
			return false, err
		}
	}

	return false, s.storage.Commit(session)
}

func (s *service) processMessageChunkEvent(ctx context.Context, eventContent *matrix.MessageChunkEventContent, senderBotUserID id.UserID, eventID id.EventID) (bool, error) {
	if err := eventContent.Verify(); err != nil {
		s.logger.Infof("Event %s, message %s from %s: invalid event content: %v", eventID, eventContent.MessageID, senderBotUserID, err)
		return true, err
	}

	session, err := s.storage.NewSession(ctx)
	if err != nil {
		err = fmt.Errorf("failed to create storage session: %w", err)
		s.logger.Errorf("Event %s, message %s: %v", eventID, eventContent.MessageID, err)
		return false, err
	}
	defer s.storage.Abort(session)

	storedChunksCount, expectedChunksCount, err := s.storage.GetChunksCount(ctx, session, eventContent.MessageID)
	if err != nil {
		err = fmt.Errorf("failed to get chunks count: %w", err)
		s.logger.Errorf("Event %s, message %s: %v", eventID, eventContent.MessageID, err)
		return false, err
	}

	newChunksCount := storedChunksCount + 1

	if newChunksCount == expectedChunksCount {
		if err := s.storage.DeleteChunkedMessage(ctx, session, eventContent.MessageID); err != nil {
			err = fmt.Errorf("failed to delete chunked message: %w", err)
			s.logger.Errorf("Event %s, message %s: %v", eventID, eventContent.MessageID, err)
			return false, err
		}
	} else {
		if err := s.storage.UpdateChunksCount(ctx, session, eventContent.MessageID, newChunksCount); err != nil {
			err = fmt.Errorf("failed to update chunks count: %w", err)
			s.logger.Errorf("Event %s, message %s: %v", eventID, eventContent.MessageID, err)
			return false, err
		}
	}
	return false, s.storage.Commit(session)
}

// TODO @evlekht implement (next ticket) // persist with db, make it durable? not just call it from event receiver?
func (s *service) banUser(_ context.Context, _ id.UserID) error {
	return nil
}
