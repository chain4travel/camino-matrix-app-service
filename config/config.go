// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

//
// ******* Parsed config *******
//

type Config struct {
	LogLevel                            string            `mapstructure:"log_level"`
	Matrix                              MatrixConfig      `mapstructure:"matrix"`
	ChainRPCURL                         string            `mapstructure:"chain_rpc_url"`
	DB                                  SQLiteDBConfig    `mapstructure:"db"`
	NetworkFeeRecipientCMAccountAddress common.Address    `mapstructure:"network_fee_recipient_cm_account_address"`
	NetworkFeeRecipientBotKey           *ecdsa.PrivateKey `mapstructure:"network_fee_recipient_bot_key"`
	MinChequeDurationUntilExpiration    *big.Int          `mapstructure:"min_cheque_duration_until_expiration"` // seconds
	CashInPeriod                        time.Duration     `mapstructure:"cash_in_period"`
}

type SQLiteDBConfig struct {
	Common        UnparsedSQLiteDBConfig
	Scheduler     UnparsedSQLiteDBConfig
	ChequeHandler UnparsedSQLiteDBConfig
}

//
// ******* Common *******
//

type MatrixConfig struct {
	HTTPPort    uint64 `mapstructure:"http_port"`
	AccessToken string `mapstructure:"access_token"`
}

//
// ******* Unparsed config *******
//

type UnparsedConfig struct {
	LogLevel                            string                 `mapstructure:"log_level"`
	Matrix                              MatrixConfig           `mapstructure:"matrix"`
	ChainRPCURL                         string                 `mapstructure:"chain_rpc_url"`
	DB                                  UnparsedSQLiteDBConfig `mapstructure:"db"`
	NetworkFeeRecipientCMAccountAddress string                 `mapstructure:"network_fee_recipient_cm_account_address"`
	NetworkFeeRecipientBotKey           string                 `mapstructure:"network_fee_recipient_bot_key"`
	MinChequeDurationUntilExpiration    uint64                 `mapstructure:"min_cheque_duration_until_expiration"` // seconds
	CashInPeriod                        int64                  `mapstructure:"cash_in_period"`                       // seconds
}

type UnparsedSQLiteDBConfig struct {
	DBPath string `mapstructure:"path"`
}

func (cfg *Config) unparse() *UnparsedConfig {
	return &UnparsedConfig{
		LogLevel:                            cfg.LogLevel,
		Matrix:                              cfg.Matrix,
		ChainRPCURL:                         cfg.ChainRPCURL,
		DB:                                  cfg.DB.Common,
		NetworkFeeRecipientCMAccountAddress: cfg.NetworkFeeRecipientCMAccountAddress.Hex(),
		NetworkFeeRecipientBotKey:           hex.EncodeToString(crypto.FromECDSA(cfg.NetworkFeeRecipientBotKey)),
		MinChequeDurationUntilExpiration:    cfg.MinChequeDurationUntilExpiration.Uint64(),
		CashInPeriod:                        int64(cfg.CashInPeriod / time.Second),
	}
}
