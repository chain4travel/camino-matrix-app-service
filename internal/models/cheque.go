package models

import (
	"github.com/chain4travel/camino-messenger-bot/pkg/cheques"
	"github.com/ethereum/go-ethereum/common"
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
