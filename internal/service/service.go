// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package service

import (
	"context"
	"errors"
	"math/big"

	"github.com/chain4travel/camino-messenger-bot/pkg/chequehandler"
	cmaccounts "github.com/chain4travel/camino-messenger-bot/pkg/cm_accounts"
	"github.com/chain4travel/camino-messenger-bot/pkg/matrix"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"go.uber.org/zap"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

var (
	_ Service = (*service)(nil)

	networkFeeBig = new(big.Int).SetUint64(100000) // nCAM

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
	cmAccounts cmaccounts.Service,
) Service {
	return &service{
		logger:                              logger,
		ethClient:                           ethClient,
		networkFeeRecipientCMAccountAddress: networkFeeRecipientCMAccountAddress,
		storage:                             storage,
		chainID:                             chainID,
		cmAccounts:                          cmAccounts,
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
	for _, evnt := range events {
		if evnt.Type.Type == matrix.EventTypeC4TMessage.Type {
			if err := evnt.Content.ParseRaw(matrix.EventTypeC4TMessage); err != nil {
				s.logger.Errorf("Failed to parse event content: %v", err)
				continue
			}

			msg, ok := evnt.Content.Parsed.(*matrix.CaminoMatrixMessage)
			if !ok {
				err := errors.New("unexpected event content type")
				s.logger.Error(err)
				continue
			}

			banSender, err := s.processMessage(ctx, msg, evnt.Sender, evnt.ID)
			if err != nil {
				return err
			}

			if banSender {
				if err := s.banUser(ctx, evnt.Sender); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// processMessage extracts network fee cheque, verifies it and stores it in the database.
// Returns true if cheque is not valid or not covering all message chunks, indicating that sender should be banned
func (s *service) processMessage(ctx context.Context, msg *matrix.CaminoMatrixMessage, senderBotUserID id.UserID, eventID id.EventID) (bool, error) {
	s.logger.Debugf("Processing message %s...", eventID)
	defer s.logger.Debugf("Finished message %s", eventID)

	if err := msg.Metadata.Verify(); err != nil {
		s.logger.Debugf("Invalid message metadata: %v", err)
		return false, err
	}

	session, err := s.storage.NewSession(ctx)
	if err != nil {
		s.logger.Errorf("Couldn't create storage session: %v", err)
		return false, err
	}
	defer s.storage.Abort(session)

	switch {
	case msg.Metadata.ChunkIndex == 0:
		cheque := msg.GetChequeFor(s.networkFeeRecipientCMAccountAddress)
		if cheque == nil {
			s.logger.Infof("event (%s) does not contain cheque for ASB owner %s", eventID, s.networkFeeRecipientCMAccountAddress)
			return true, nil
		}

		if err := s.chequeHandler.VerifyCheque(ctx, cheque, matrix.AddressFromUserID(senderBotUserID), networkFeeBig); err != nil {
			s.logger.Infof("Failed to verify cheque: %v", err)
			return true, nil
		}

		if err := s.storage.InsertChunkedMessage(ctx, session, msg.Metadata.RequestID, msg.Metadata.NumberOfChunks); err != nil {
			s.logger.Errorf("Failed to store message chunk: %v", err)
			return false, err
		}

	case msg.Metadata.ChunkIndex == msg.Metadata.NumberOfChunks-1:
		if err := s.storage.DeleteChunkedMessage(ctx, session, msg.Metadata.RequestID); err != nil {
			s.logger.Errorf("Failed to delete message chunk: %v", err)
			return false, err
		}

	default: // Middle chunk, first chunk is already stored
		_, maxChunksNumber, err := s.storage.GetChunksNumbers(ctx, session, msg.Metadata.RequestID)
		if err != nil {
			s.logger.Errorf("Couldn't create storage session: %v", err)
			return false, err
		}

		if msg.Metadata.ChunkIndex > maxChunksNumber-1 {
			s.logger.Infof("event (%s) chunk index %d is out of range (0-%d)", eventID, msg.Metadata.ChunkIndex, maxChunksNumber-1)
			return true, nil
		}

		if err := s.storage.AddMessageChunk(ctx, session, msg.Metadata.RequestID); err != nil {
			s.logger.Errorf("Failed to add message chunk: %v", err)
			return false, err
		}
	}

	return false, s.storage.Commit(session)
	// TODO @evlekht do cash in amount threshold reached? store unpaid amount in cheque?
}

// TODO @evlekht implement (next ticket) // persist with db, make it durable? not just call it from event receiver?
func (s *service) banUser(_ context.Context, _ id.UserID) error {
	return nil
}
