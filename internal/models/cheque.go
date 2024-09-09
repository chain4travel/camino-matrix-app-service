package models

import (
	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

type Cheque struct {
	// tTxs.Cheque
	ChequebookID string
	Signature    [secp256k1.SignatureLen]byte
	TxID         ids.ID
	// Status       tStatus.Status
}

// func (ch Cheque) String() string {
// 	return fmt.Sprintf("%s-@-%s-->%s:amt-%d:n-%d", ch.Agent, ch.Issuer, ch.Beneficiary, ch.Amount, ch.SerialID)
// }

// func (ch *Cheque) Verify() error {
// 	switch {
// 	case ch.Issuer == ch.Beneficiary:
// 		return fmt.Errorf("issue is the same as beneficiary")
// 	case ch.SerialID == 0:
// 		return fmt.Errorf("serialID can't be 0")
// 	}
// 	return nil
// }

// func (ch *Cheque) VerifyWithPrevious(previous *Cheque) error {
// 	switch {
// 	case previous == nil:
// 		return nil
// 	case previous.Amount > ch.Amount:
// 		return errors.New("previous cheque amount is greater than new cheque amount")
// 	case previous.SerialID >= ch.SerialID:
// 		return errors.New("previous cheque serialID is greater or equal to new cheque serialID")
// 	}
// 	return nil
// }

func (ch *Cheque) Credential() *secp256k1fx.Credential {
	return &secp256k1fx.Credential{
		Sigs: [][65]byte{ch.Signature},
	}
}
