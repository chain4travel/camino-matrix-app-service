// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"crypto/ecdsa"
	"encoding/hex"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

// ******* Parsed config *******
//
//

type Config struct {
	LogLevel                   string            `mapstructure:"log_level"`
	CChainRPCURL               string            `mapstructure:"c_chain_rpc_url"`
	HTTPPort                   string            `mapstructure:"http_port"`
	MatrixAccessToken          string            `mapstructure:"matrix_access_token"`
	DB                         SQLiteDBConfig    `mapstructure:"db"`
	CMAccountAddress           common.Address    `mapstructure:"cm_account_address"`
	NetworkFeeRecipientKey     *ecdsa.PrivateKey `mapstructure:"network_fee_recipient_key"`
	MinDurationUntilExpiration uint64            `mapstructure:"min_duration_until_expiration"` // seconds
	CashInPeriod               time.Duration     `mapstructure:"cash_in_period"`
}

type SQLiteDBConfig struct {
	Common        UnparsedSQLiteDBConfig
	Scheduler     UnparsedSQLiteDBConfig
	ChequeHandler UnparsedSQLiteDBConfig
}

// ******* Unparsed config *******
//
//

type UnparsedConfig struct {
	LogLevel                   string                 `mapstructure:"log_level"`
	CChainRPCURL               string                 `mapstructure:"c_chain_rpc_url"`
	HTTPPort                   string                 `mapstructure:"http_port"`
	MatrixAccessToken          string                 `mapstructure:"matrix_access_token"`
	DB                         UnparsedSQLiteDBConfig `mapstructure:"db"`
	CMAccountAddress           string                 `mapstructure:"cm_account_address"`
	NetworkFeeRecipientKey     string                 `mapstructure:"network_fee_recipient_key"`
	MinDurationUntilExpiration uint64                 `mapstructure:"min_duration_until_expiration"` // seconds
	CashInPeriod               uint64                 `mapstructure:"cash_in_period"`                // seconds
}

type UnparsedSQLiteDBConfig struct {
	DBPath         string `mapstructure:"path"`
	MigrationsPath string `mapstructure:"migrations_path"`
}

func (cfg *Config) unparse() *UnparsedConfig {
	return &UnparsedConfig{
		LogLevel:                   cfg.LogLevel,
		CChainRPCURL:               cfg.CChainRPCURL,
		DB:                         cfg.DB.Common,
		HTTPPort:                   cfg.HTTPPort,
		MatrixAccessToken:          cfg.MatrixAccessToken,
		CMAccountAddress:           cfg.CMAccountAddress.Hex(),
		NetworkFeeRecipientKey:     hex.EncodeToString(crypto.FromECDSA(cfg.NetworkFeeRecipientKey)),
		MinDurationUntilExpiration: cfg.MinDurationUntilExpiration,
		CashInPeriod:               uint64(cfg.CashInPeriod / time.Second),
	}
}
