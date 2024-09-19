package models

import (
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

type Chequebook struct {
	cheques.SignedCheque
	ChequebookID common.Hash
	TxID         common.Hash
	Status       ChequeTxStatus
}

func ChequeTxStatusFromTxStatus(txStatus uint64) ChequeTxStatus {
	if txStatus == types.ReceiptStatusSuccessful {
		return ChequeTxStatusAccepted
	}
	return ChequeTxStatusRejected
}
