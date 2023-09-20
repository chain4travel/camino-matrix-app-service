package matrix

import (
	"camino-synapse-appservice/internal/models"
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	tTxs "github.com/ava-labs/avalanchego/vms/touristicvm/txs"
	"go.uber.org/zap"
	"maunium.net/go/mautrix"
	"maunium.net/go/mautrix/event"
	"maunium.net/go/mautrix/id"
)

const c4tMessageType = "m.room.c4t-msg"

var (
	eventC4TMessage = event.Type{Type: c4tMessageType, Class: event.MessageEventType}

	_ Client = (*client)(nil)
)

type cheque struct {
	Issuer      ids.ShortID `json:"issuer"`
	Agent       ids.ShortID `json:"agent"`
	Beneficiary ids.ShortID `json:"beneficiary"`
	Amount      uint64      `json:"amount"`
	SerialID    uint64      `json:"serialID"`
	Signature   string      `json:"signature"`
}

type c4tMessageEventContent struct {
	MsgType event.MessageType `json:"msgtype,omitempty"`
	Body    string            `json:"body"`
	Cheques []cheque          `json:"cheques"`
}

type Client interface {
	GetC4TMessageCheques(evnt event.Event) ([]models.SignedCheque, error)
}

func NewClient(ctx context.Context, logger *zap.SugaredLogger, homeserver, accessToken, userID string) (Client, error) {
	matrixClient, err := mautrix.NewClient(homeserver, id.UserID(userID), accessToken)
	if err != nil {
		logger.Error(err)
		return &client{}, err
	}

	event.TypeMap[eventC4TMessage] = reflect.TypeOf(c4tMessageEventContent{})

	return &client{logger: logger, matrix: matrixClient}, nil
}

type client struct {
	logger *zap.SugaredLogger
	matrix *mautrix.Client
}

func (c *client) GetC4TMessageCheques(evnt event.Event) ([]models.SignedCheque, error) {
	if evnt.Type.Type != c4tMessageType {
		err := fmt.Errorf("wrong event type: expected %s, but got %s", c4tMessageType, evnt.Type.Type)
		c.logger.Error(err)
		return nil, err
	}
	if err := evnt.Content.ParseRaw(eventC4TMessage); err != nil {
		c.logger.Error(err)
		return nil, err
	}
	msg, ok := evnt.Content.Parsed.(*c4tMessageEventContent)
	if !ok {
		err := errors.New("unexpected message type")
		c.logger.Error(err)
		return nil, err
	}
	return c.signedCheques(msg.Cheques)
}

func (c *client) signedCheques(cheques []cheque) ([]models.SignedCheque, error) {
	signedCheques := make([]models.SignedCheque, len(cheques))
	for i, cheque := range cheques {
		signature, err := formatting.Decode(formatting.Hex, string(cheque.Signature))
		if err != nil {
			err := fmt.Errorf("failed to decode cheque signature from hex: %v", err)
			c.logger.Error(err)
			return nil, err
		}
		if len(signature) != secp256k1.SignatureLen {
			err := fmt.Errorf("wrong cheque signature len: expected %d, but got %d", secp256k1.SignatureLen, len(signature))
			c.logger.Error(err)
			return nil, err
		}

		signedCheques[i] = models.SignedCheque{
			Cheque: tTxs.Cheque{
				Issuer:      cheque.Issuer,
				Agent:       cheque.Agent,
				Beneficiary: cheque.Beneficiary,
				Amount:      cheque.Amount,
				SerialID:    cheque.SerialID,
			},
			Credential: &secp256k1fx.Credential{
				Sigs: make([][secp256k1.SignatureLen]byte, 1),
			},
		}
		copy(signedCheques[i].Credential.Sigs[0][:], signature)
	}
	return signedCheques, nil
}

func IsC4TMessage(evnt *event.Event) bool {
	return evnt.Type.Type == c4tMessageType
}
