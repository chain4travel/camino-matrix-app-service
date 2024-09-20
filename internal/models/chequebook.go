package models

import (
	"fmt"

	"github.com/chain4travel/camino-messenger-bot/pkg/cheques"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type ChequeTxStatus uint8

const (
	ChequeTxStatusUnknown ChequeTxStatus = iota
	ChequeTxStatusProcessing
	ChequeTxStatusAccepted
	ChequeTxStatusRejected
)

func (c ChequeTxStatus) String() string {
	switch c {
	case ChequeTxStatusUnknown:
		return "unknown"
	case ChequeTxStatusProcessing:
		return "processing"
	case ChequeTxStatusAccepted:
		return "accepted"
	case ChequeTxStatusRejected:
		return "rejected"
	default:
		return fmt.Sprintf("unknown status: %d", c)
	}
}

func ChequeTxStatusFromTxStatus(txStatus uint64) ChequeTxStatus {
	if txStatus == types.ReceiptStatusSuccessful {
		return ChequeTxStatusAccepted
	}
	return ChequeTxStatusRejected
}

type Chequebook struct {
	cheques.SignedCheque
	ChequebookID common.Hash
	TxID         common.Hash
	Status       ChequeTxStatus
}

func (c Chequebook) String() string {
	return fmt.Sprintf("{ID: %s, txID %s, status: %s, cheque: %+v}", c.ChequebookID.Hex(), c.TxID.Hex(), c.Status, c.Cheque)
}
