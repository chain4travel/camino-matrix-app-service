package matrix

import (
	"context"
	"errors"
	"fmt"
	"reflect"

	"github.com/chain4travel/camino-synapse-app-service/internal/models"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/utils/formatting/address"
	"github.com/ava-labs/avalanchego/utils/json"
	"github.com/ava-labs/avalanchego/vms/touristicvm/txs"
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
	Issuer      string      `json:"issuer"`
	Agent       string      `json:"agent"`
	Beneficiary string      `json:"beneficiary"`
	Amount      json.Uint64 `json:"amount"`
	SerialID    json.Uint64 `json:"serialID"`
	Signature   string      `json:"signature"`
}

type c4tMessageEventContent struct {
	MsgType event.MessageType `json:"msgtype,omitempty"`
	Body    string            `json:"body"`
	Cheques []cheque          `json:"cheques"`
}

type Client interface {
	ParseC4TMessageCheques(evnt *event.Event) ([]models.Cheque, error)
}

func NewClient(ctx context.Context, logger *zap.SugaredLogger, homeserver, accessToken, userID string, networkID uint32) (Client, error) {
	matrixClient, err := mautrix.NewClient(homeserver, id.UserID(userID), accessToken)
	if err != nil {
		logger.Error(err)
		return &client{}, err
	}

	event.TypeMap[eventC4TMessage] = reflect.TypeOf(c4tMessageEventContent{})

	return &client{
		logger:    logger,
		matrix:    matrixClient,
		networkID: networkID,
	}, nil
}

type client struct {
	logger    *zap.SugaredLogger
	matrix    *mautrix.Client
	networkID uint32
}

func (c *client) ParseC4TMessageCheques(evnt *event.Event) ([]models.Cheque, error) {
	if !IsC4TMessage(evnt) {
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
		err := errors.New("unexpected event content type")
		c.logger.Error(err)
		return nil, err
	}
	return c.signedCheques(msg.Cheques)
}

func (c *client) signedCheques(cheques []cheque) ([]models.Cheque, error) {
	signedCheques := make([]models.Cheque, len(cheques))
	for i, cheque := range cheques {
		signature, err := formatting.Decode(formatting.Hex, cheque.Signature)
		if err != nil {
			c.logger.Debugf("failed to decode cheque signature from hex: %v", err)
			c.logger.Debugf("Prefixing encoded signature with 0x")
			signature, err = formatting.Decode(formatting.Hex, "0x"+cheque.Signature)
			if err != nil {
				err := fmt.Errorf("failed to decode prefixed cheque signature from hex: %v", err)
				c.logger.Error(err)
				return nil, err
			}
		}
		if len(signature) != secp256k1.SignatureLen {
			err := fmt.Errorf("wrong cheque signature len: expected %d, but got %d", secp256k1.SignatureLen, len(signature))
			c.logger.Error(err)
			return nil, err
		}

		_, hrp, addrBytes, err := address.Parse(cheque.Issuer)
		if err != nil {
			err := fmt.Errorf("failed to parse cheque issuer address: %v", err)
			c.logger.Error(err)
			return nil, err
		}
		issuer, err := ids.ToShortID(addrBytes)
		if err != nil {
			err := fmt.Errorf("failed to parse cheque issuer address: %v", err)
			c.logger.Error(err)
			return nil, err
		}
		if hrp != constants.GetHRP(c.networkID) {
			err := fmt.Errorf("failed to parse cheque issuer address: expected hrp %s, but got %s",
				constants.GetHRP(c.networkID), hrp)
			c.logger.Error(err)
			return nil, err
		}

		_, hrp, addrBytes, err = address.Parse(cheque.Beneficiary)
		if err != nil {
			err := fmt.Errorf("failed to parse cheque issuer address: %v", err)
			c.logger.Error(err)
			return nil, err
		}
		beneficiary, err := ids.ToShortID(addrBytes)
		if err != nil {
			err := fmt.Errorf("failed to parse cheque beneficiary address: %v", err)
			c.logger.Error(err)
			return nil, err
		}
		if hrp != constants.GetHRP(c.networkID) {
			err := fmt.Errorf("failed to parse cheque beneficiary address: expected hrp %s, but got %s",
				constants.GetHRP(c.networkID), hrp)
			c.logger.Error(err)
			return nil, err
		}

		agent, err := ids.ShortFromString(cheque.Agent)
		if err != nil {
			err := fmt.Errorf("failed to parse cheque beneficiary address: %v", err)
			c.logger.Error(err)
			return nil, err
		}

		signedCheques[i] = models.Cheque{
			ChequebookID: issuer.String() + beneficiary.String() + agent.String(),
			Cheque: txs.Cheque{
				Issuer:      issuer,
				Agent:       agent,
				Beneficiary: beneficiary,
				Amount:      uint64(cheque.Amount),
				SerialID:    uint64(cheque.SerialID),
			},
		}
		copy(signedCheques[i].Signature[:], signature)
	}
	return signedCheques, nil
}

func IsC4TMessage(evnt *event.Event) bool {
	return evnt.Type.Type == c4tMessageType
}
