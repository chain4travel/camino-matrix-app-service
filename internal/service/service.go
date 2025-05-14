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
	"github.com/chain4travel/camino-messenger-bot/v11/pkg/metadata"
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

	if err := verifyMetadata(msg.Metadata); err != nil {
		s.logger.Infof("Event %s, request %s: invalid message metadata: %v", eventID, msg.Metadata.RequestID, err)
		return true, err
	}

	session, err := s.storage.NewSession(ctx)
	if err != nil {
		err = fmt.Errorf("failed to create storage session: %w", err)
		s.logger.Errorf("Event %s, request %s: %v", eventID, msg.Metadata.RequestID, err)
		return false, err
	}
	defer s.storage.Abort(session)

	switch {
	case msg.Metadata.ChunkIndex == 0:
		cheque := msg.GetChequeFor(s.networkFeeRecipientCMAccountAddress)
		if cheque == nil {
			s.logger.Infof("Event %s, request %s: cheque not found for ASB owner %s", eventID, msg.Metadata.RequestID, s.networkFeeRecipientCMAccountAddress)
			return true, nil
		}

		if err := s.chequeHandler.VerifyCheque(ctx, cheque, matrix.AddressFromUserID(senderBotUserID), config.NetworkFee); err != nil {
			s.logger.Infof("Event %s, request %s: failed to verify cheque: %v", eventID, msg.Metadata.RequestID, err)
			return true, nil
		}

		if err := s.storage.InsertChunkedMessage(ctx, session, msg.Metadata.RequestID, msg.Metadata.NumberOfChunks); err != nil {
			err = fmt.Errorf("failed to insert chunked message: %w", err)
			s.logger.Errorf("Event %s, request %s: %v", eventID, msg.Metadata.RequestID, err)
			return false, err
		}

	case msg.Metadata.ChunkIndex == msg.Metadata.NumberOfChunks-1:
		if err := s.storage.DeleteChunkedMessage(ctx, session, msg.Metadata.RequestID); err != nil {
			err = fmt.Errorf("failed to delete chunked message: %w", err)
			s.logger.Errorf("Event %s, request %s: %v", eventID, msg.Metadata.RequestID, err)
			return false, err
		}

	default: // Middle chunk, first chunk is already stored
		_, maxChunksNumber, err := s.storage.GetChunksNumbers(ctx, session, msg.Metadata.RequestID)
		if err != nil {
			err = fmt.Errorf("failed to get chunks numbers: %w", err)
			s.logger.Errorf("Event %s, request %s: %v", eventID, msg.Metadata.RequestID, err)
			return false, err
		}

		if msg.Metadata.ChunkIndex > maxChunksNumber-1 {
			s.logger.Infof("Event %s, request %s: chunk index %d is out of range (0-%d)", eventID, msg.Metadata.RequestID, msg.Metadata.ChunkIndex, maxChunksNumber-1)
			return true, nil
		}

		if err := s.storage.AddMessageChunk(ctx, session, msg.Metadata.RequestID); err != nil {
			err = fmt.Errorf("failed to add message chunk: %w", err)
			s.logger.Errorf("Event %s, request %s: %v", eventID, msg.Metadata.RequestID, err)
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

func verifyMetadata(m metadata.Metadata) error {
	if m.RequestID == "" {
		return fmt.Errorf("request id is empty")
	}
	if len(m.Cheques) == 0 {
		return fmt.Errorf("no cheques")
	}
	if m.NumberOfChunks == 0 {
		return fmt.Errorf("number of chunks is zero")
	}
	if m.ChunkIndex >= m.NumberOfChunks {
		return fmt.Errorf("chunk index %d is greater than number of chunks %d", m.ChunkIndex, m.NumberOfChunks)
	}
	// TODO @evlekht do we need to verify sender cm account?
	// TODO verify cheque signer bot with this cm account later (maybe already verified in cheque verification)?
	// TODO verify cheque from cm account to be equal to message metadata? should cmb verify this as well?
	return nil
}
