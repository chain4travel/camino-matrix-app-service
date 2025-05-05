// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const envPrefix = "CMB"

var (
	_ Reader = (*reader)(nil)

	errInvalidRawConfig = errors.New("invalid raw config")
)

type Reader interface {
	ReadConfig() (*Config, error)
}

// Returns a new config reader.
func NewConfigReader(flags *pflag.FlagSet, logger *zap.SugaredLogger) (Reader, error) {
	return &reader{
		viper:  viper.New(),
		flags:  flags,
		logger: logger,
	}, nil
}

type reader struct {
	viper  *viper.Viper
	logger *zap.SugaredLogger
	flags  *pflag.FlagSet
}

func (cr *reader) ReadConfig() (*Config, error) {
	cr.viper.SetEnvPrefix(envPrefix)
	cr.viper.AutomaticEnv()
	cr.viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	if err := cr.viper.BindPFlags(cr.flags); err != nil {
		cr.logger.Errorf("Error binding flags: %s", err)
		return nil, err
	}

	configPath := cr.viper.GetString(flagKeyConfig)
	if configPath == "" {
		err := errors.New("config path is empty")
		cr.logger.Error(err)
		return nil, err
	}
	cr.viper.SetConfigFile(configPath)

	if err := cr.viper.ReadInConfig(); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			cr.logger.Errorf("Error reading config file: %s", err)
			return nil, err
		}
		cr.logger.Info("Config file not found")
	}

	cfg := &UnparsedConfig{}
	if err := cr.viper.Unmarshal(cfg); err != nil {
		cr.logger.Error(err)
		return nil, err
	}

	parsedCfg, err := cr.parseConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errInvalidRawConfig, err)
	}

	return parsedCfg, nil
}

func (cr *reader) parseConfig(cfg *UnparsedConfig) (*Config, error) {
	networkFeeRecipientECDSAKey, err := crypto.HexToECDSA(cfg.NetworkFeeRecipientKey)
	if err != nil {
		cr.logger.Error(err)
		return nil, err
	}

	if !common.IsHexAddress(cfg.CMAccountAddress) {
		err := errors.New("invalid CM account address")
		cr.logger.Error(err)
		return nil, err
	}

	return &Config{
		CashInPeriod:      time.Duration(cfg.CashInPeriod) * time.Second,
		CChainRPCURL:      cfg.CChainRPCURL,
		HTTPPort:          cfg.HTTPPort,
		MatrixAccessToken: cfg.MatrixAccessToken,
		LogLevel:          cfg.LogLevel,
		DB: SQLiteDBConfig{
			Common: cfg.DB,
			Scheduler: UnparsedSQLiteDBConfig{
				DBPath:         cfg.DB.DBPath + "/scheduler",
				MigrationsPath: cfg.DB.MigrationsPath + "/scheduler",
			},
			ChequeHandler: UnparsedSQLiteDBConfig{
				DBPath:         cfg.DB.DBPath + "/cheque_handler",
				MigrationsPath: cfg.DB.MigrationsPath + "/cheque_handler",
			},
		},
		CMAccountAddress:           common.HexToAddress(cfg.CMAccountAddress),
		NetworkFeeRecipientKey:     networkFeeRecipientECDSAKey,
		MinDurationUntilExpiration: cfg.MinDurationUntilExpiration,
	}, nil
}
