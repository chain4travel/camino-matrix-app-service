package node

import (
	"context"

	"github.com/chain4travel/camino-synapse-app-service/internal/logger"

	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/vms/platformvm"
)

var _ Client = (*client)(nil)

type Client interface {
	// CashOutTx(cheque *models.Cheque) (ids.ID, error)
	// GetTxStatus(txID ids.ID) (status tStatus.Status, reason string, err error)
	NetworkID() uint32
}

func NewClient(ctx context.Context, nodeURI string, logger logger.Logger) (Client, error) {
	pClient := platformvm.NewClient(nodeURI)

	nodeCfg, err := pClient.GetConfiguration(ctx)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	return &client{
		logger:    logger,
		uri:       nodeURI,
		networkID: uint32(nodeCfg.NetworkID),
		hrp:       constants.GetHRP(uint32(nodeCfg.NetworkID)),
	}, nil
}

type client struct {
	logger logger.Logger

	uri       string // TODO@ is it needed?
	networkID uint32 // TODO@ is it needed?
	hrp       string // TODO@ is it needed?
}

func (c *client) NetworkID() uint32 {
	return c.networkID
}

// func (c *client) CashOutTx(cheque *models.Cheque) (ids.ID, error) {
// 	c.logger.Debug("Creating cashOutTx...")
// 	c.logger.Debug("Requesting T-chain SpendWithWrapper...")
// 	ins, outs, _, _, err := c.T.SpendWithWrapper(
// 		context.Background(),
// 		cheque.Issuer,
// 		cheque.Agent,
// 		cheque.Beneficiary,
// 		cheque.Amount, 0,
// 		locked.StateUnlocked,
// 		pAPI.Owner{},
// 	)
// 	if err != nil {
// 		c.logger.Error(err)
// 		return ids.Empty, err
// 	}

// 	utx := &tTxs.CashoutChequeTx{
// 		BaseTx: tTxs.BaseTx{BaseTx: avax.BaseTx{
// 			NetworkID:    c.networkID,
// 			BlockchainID: c.tChainID,
// 			Ins:          ins,
// 			Outs:         outs,
// 		}},
// 		Cheque: tTxs.SignedCheque{
// 			Cheque: cheque.Cheque,
// 			Auth:   cheque.Credential(),
// 		},
// 	}

// 	tx, err := tTxs.NewSigned(utx, tTxs.Codec, nil)
// 	if err != nil {
// 		c.logger.Error(err)
// 		return ids.Empty, err
// 	}

// 	txEncodedBytes, err := formatting.Encode(formatting.Hex, tx.Bytes())
// 	if err != nil {
// 		c.logger.Error(err)
// 		return ids.Empty, err
// 	}
// 	c.logger.Debug(txEncodedBytes)
// 	c.logger.Debugf("txID: %s", tx.ID())

// 	resp, err := c.T.GetTxStatus(context.Background(), tx.ID())
// 	if err != nil {
// 		c.logger.Error(err)
// 		return ids.Empty, err
// 	}

// 	if resp.Status != tStatus.Unknown {
// 		c.logger.Debug("Found existing cashOutTx")
// 		return tx.ID(), nil
// 	}

// 	c.logger.Debug("Issuing cashOutTx...")
// 	if _, err := c.T.IssueTx(context.Background(), tx.Bytes()); err != nil {
// 		c.logger.Error(err)
// 		return ids.Empty, err
// 	}
// 	c.logger.Debug("CashOutTx issued")
// 	return tx.ID(), nil
// }

// func (c *client) GetTxStatus(txID ids.ID) (status tStatus.Status, reason string, err error) {
// 	c.logger.Debugf("Getting status of tx %s", txID)
// 	resp, err := c.T.GetTxStatus(context.Background(), txID)
// 	if err != nil {
// 		c.logger.Error(err)
// 		return tStatus.Unknown, "", err
// 	}
// 	c.logger.Debugf("Tx %s status: %s (%s)", txID, resp.Status, resp.Reason)
// 	return resp.Status, resp.Reason, nil
// }
