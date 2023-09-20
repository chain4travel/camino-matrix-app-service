package models

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/ava-labs/avalanchego/ids"
	"github.com/ava-labs/avalanchego/utils/crypto/secp256k1"
	"github.com/ava-labs/avalanchego/utils/hashing"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
	tTxs "github.com/ava-labs/avalanchego/vms/touristicvm/txs"
)

var emptyChequeRecord = &ChequeRecord{Credential: &secp256k1fx.Credential{}}

type Trio struct {
	Agent            ids.ShortID  `serialize:"true"`
	Issuer           ids.ShortID  `serialize:"true"`
	Beneficiary      ids.ShortID  `serialize:"true"`
	LastAdded        ChequeRecord `serialize:"true"`
	LastIssuedWithTx ChequeRecord `serialize:"true"`
	LastCashedOut    ChequeRecord `serialize:"true"`
}

func (t *Trio) IsCashedOut() bool {
	return t.LastAdded.Equal(&t.LastCashedOut)
}

func (t *Trio) LastAddedCheque() *SignedCheque {
	return &SignedCheque{
		Cheque: tTxs.Cheque{
			Agent:       t.Agent,
			Issuer:      t.Issuer,
			Beneficiary: t.Beneficiary,
			Amount:      t.LastAdded.Amount,
			SerialID:    t.LastAdded.SerialID,
		},
		Credential: t.LastAdded.Credential,
	}
}

type SignedCheque struct {
	tTxs.Cheque `serialize:"true"`
	Credential  *secp256k1fx.Credential `serialize:"true"`
}

func (ch *SignedCheque) String() string {
	return fmt.Sprintf("%s@%s->%s:amt-%d:n-%d", ch.Agent, ch.Issuer, ch.Beneficiary, ch.Amount, ch.SerialID)
}

func (ch *SignedCheque) Verify() error {
	if len(ch.Credential.Sigs) != 1 {
		return fmt.Errorf("wrong number of sigs: expected 1, got %d", len(ch.Credential.Sigs))
	}

	if ch.Issuer == ch.Beneficiary {
		return fmt.Errorf("issue is the same as beneficiary")
	}

	cheque := ch.Issuer.String() + ch.Beneficiary.String() + strconv.FormatUint(ch.Amount, 10) + strconv.FormatUint(ch.SerialID, 10)
	chequeHash := hashing.ComputeHash256([]byte(cheque))

	factory := secp256k1.Factory{}
	pubKey, err := factory.RecoverHashPublicKey(chequeHash, ch.Credential.Sigs[0][:])
	if err != nil {
		return fmt.Errorf("failed to recover public key from cheque signature: %v", err)
	}

	if pubKey.Address() != ch.Issuer {
		return fmt.Errorf("signed by %s, but expected %s", pubKey.Address().String(), ch.Issuer.String())
	}

	return nil
}

func (ch *SignedCheque) VerifyWithPrevious(previous *ChequeRecord) error {
	if previous == nil {
		return nil
	}
	switch {
	case previous.Amount > ch.Amount:
		return errors.New("previous cheque amount is greater than new cheque amount")
	case previous.SerialID >= ch.SerialID:
		return errors.New("previous cheque serialID is greater or equal to new cheque serialID")
	}
	return nil
}

func (ch *SignedCheque) IsNewerThan(old *ChequeRecord) bool {
	return old == nil || old == emptyChequeRecord || old.SerialID < ch.SerialID
}

func (ch *SignedCheque) TrioID() string {
	return ch.Issuer.String() + ch.Beneficiary.String() + ch.Agent.String()
}

func (ch *SignedCheque) Trio() *Trio {
	return &Trio{
		Issuer:           ch.Issuer,
		Agent:            ch.Agent,
		Beneficiary:      ch.Beneficiary,
		LastAdded:        ChequeRecord{Credential: &secp256k1fx.Credential{}},
		LastIssuedWithTx: ChequeRecord{Credential: &secp256k1fx.Credential{}},
		LastCashedOut:    ChequeRecord{Credential: &secp256k1fx.Credential{}},
	}
}

func (ch *SignedCheque) ChequeRecord() ChequeRecord {
	return ChequeRecord{
		Amount:     ch.Amount,
		SerialID:   ch.SerialID,
		Credential: ch.Credential,
	}
}

type ChequeRecord struct {
	Amount     uint64                  `serialize:"true"`
	SerialID   uint64                  `serialize:"true"`
	Credential *secp256k1fx.Credential `serialize:"true"`
}

func (chr *ChequeRecord) Equal(other *ChequeRecord) bool {
	if len(chr.Credential.Sigs) != len(other.Credential.Sigs) {
		return false
	}
	for i := range chr.Credential.Sigs {
		if chr.Credential.Sigs[i] != other.Credential.Sigs[i] {
			return false
		}
	}
	return chr.Amount == other.Amount && chr.SerialID == other.SerialID
}
