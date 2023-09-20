package node

import (
	"context"

	"github.com/chain4travel/camino-synapse-app-service/internal/models"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/constants"
	"github.com/ava-labs/avalanchego/utils/formatting"
	"github.com/ava-labs/avalanchego/vms/components/avax"
	"github.com/ava-labs/avalanchego/vms/platformvm"
	pAPI "github.com/ava-labs/avalanchego/vms/platformvm/api"
	"github.com/ava-labs/avalanchego/vms/touristicvm"
	"github.com/ava-labs/avalanchego/vms/touristicvm/locked"
	tStatus "github.com/ava-labs/avalanchego/vms/touristicvm/status"
	tTxs "github.com/ava-labs/avalanchego/vms/touristicvm/txs"
	"go.uber.org/zap"
)

var _ Client = (*client)(nil)

type Client interface {
	CashOutTx(cheque *models.SignedCheque) (ids.ID, error)
	GetTxStatus(txID ids.ID) (status tStatus.Status, reason string, err error)
}

func NewClient(ctx context.Context, nodeURI string, logger *zap.SugaredLogger) (Client, error) {
	pClient := platformvm.NewClient(nodeURI)

	nodeCfg, err := pClient.GetConfiguration(ctx)
	if err != nil {
		logger.Error(err)
		return nil, err
	}

	tChainID := ids.ID{}
	for _, blockchain := range nodeCfg.Blockchains {
		if blockchain.Name == "T-Chain" {
			tChainID = blockchain.ID
			break
		}
	}

	return &client{
		logger:    logger,
		T:         touristicvm.NewClient(nodeURI),
		uri:       nodeURI,
		networkID: uint32(nodeCfg.NetworkID),
		tChainID:  tChainID,
		hrp:       constants.GetHRP(uint32(nodeCfg.NetworkID)),
	}, nil
}

type client struct {
	logger *zap.SugaredLogger

	T touristicvm.Client

	uri       string
	tChainID  ids.ID
	networkID uint32
	hrp       string
}

func (c *client) CashOutTx(cheque *models.SignedCheque) (ids.ID, error) {
	c.logger.Debug("Creating cashOutTx...")
	c.logger.Debug("Requesting T-chain SpendWithWrapper...")
	ins, outs, _, _, err := c.T.SpendWithWrapper(
		context.Background(),
		cheque.Issuer,
		cheque.Agent,
		cheque.Beneficiary,
		cheque.Amount, 0,
		locked.StateUnlocked,
		pAPI.Owner{},
	)
	if err != nil {
		c.logger.Error(err)
		return ids.Empty, err
	}

	utx := &tTxs.CashoutChequeTx{
		BaseTx: tTxs.BaseTx{BaseTx: avax.BaseTx{
			NetworkID:    c.networkID,
			BlockchainID: c.tChainID,
			Ins:          ins,
			Outs:         outs,
		}},
		Cheque: tTxs.SignedCheque{
			Cheque: cheque.Cheque,
			Auth:   cheque.Credential,
		},
	}

	avax.SortTransferableInputs(utx.Ins)
	avax.SortTransferableOutputs(utx.Outs, tTxs.Codec)
	tx, err := tTxs.NewSigned(utx, tTxs.Codec, nil)
	if err != nil {
		c.logger.Error(err)
		return ids.Empty, err
	}

	txEncodedBytes, err := formatting.Encode(formatting.Hex, tx.Bytes())
	if err != nil {
		c.logger.Error(err)
		return ids.Empty, err
	}
	c.logger.Debug(txEncodedBytes)
	c.logger.Debugf("txID: %s", tx.ID())

	resp, err := c.T.GetTxStatus(context.Background(), tx.ID())
	if err != nil {
		c.logger.Error(err)
		return ids.Empty, err
	}

	if resp.Status != tStatus.Unknown {
		c.logger.Debug("Found existing cashOutTx")
		return tx.ID(), nil
	}

	c.logger.Debug("Issuing cashOutTx...")
	if _, err := c.T.IssueTx(context.Background(), tx.Bytes()); err != nil {
		c.logger.Error(err)
		return ids.Empty, err
	}
	c.logger.Debug("CashOutTx issued")
	return tx.ID(), nil
}

func (c *client) GetTxStatus(txID ids.ID) (status tStatus.Status, reason string, err error) {
	c.logger.Debugf("GetTxStatus %s", txID)
	resp, err := c.T.GetTxStatus(context.Background(), txID)
	if err != nil {
		c.logger.Error(err)
		return tStatus.Unknown, "", err
	}
	return resp.Status, resp.Reason, nil
}
