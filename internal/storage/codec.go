package storage

import (
	"math"

	avaxCodec "github.com/ava-labs/avalanchego/codec"
	"github.com/ava-labs/avalanchego/codec/linearcodec"
	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/ava-labs/avalanchego/vms/secp256k1fx"
)

const codecVersion = 0

var codec avaxCodec.Manager

func init() {
	// Create default codec and manager
	c := linearcodec.NewDefault()
	codec = avaxCodec.NewDefaultManager()
	gc := linearcodec.NewCustomMaxLength(math.MaxInt32)

	errs := wrappers.Errs{}
	for _, c := range []linearcodec.Codec{c, gc} {
		errs.Add(registerUnsignedTxsTypes(c))
	}
	errs.Add(
		codec.RegisterCodec(codecVersion, c),
	)
	if errs.Errored() {
		panic(errs.Err)
	}
}

func registerUnsignedTxsTypes(targetCodec avaxCodec.Registry) error {
	errs := wrappers.Errs{}
	errs.Add(
		targetCodec.RegisterType(&secp256k1fx.Credential{}),
	)
	return errs.Err
}
