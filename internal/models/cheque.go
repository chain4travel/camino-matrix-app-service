package models

import (
	"github.com/chain4travel/camino-messenger-bot/pkg/cheques"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
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

func ChequebookID(cheque *cheques.SignedCheque) common.Hash {
	return crypto.Keccak256Hash(
		cheque.FromCMAccount.Bytes(),
		cheque.ToCMAccount.Bytes(),
		cheque.ToBot.Bytes(),
	)
}
