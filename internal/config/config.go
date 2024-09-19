package config

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	configFileName = "camino-synapse-app-service"

	configFlagKey = "config"

	cashInPeriodKey             = "cash_in_period"
	caminoNodeHostKey           = "camino_node_host"
	httPortKey                  = "http_port"
	matrixAccessTokenKey        = "matrix_access_token"
	logLevelKey                 = "log_level"
	dbPathKey                   = "db_path"
	dbNameKey                   = "db_name"
	migrationsPathKey           = "migrations_path"
	cmAccountContractAddressKey = "cm_account_contract_address"
	networkFeeRecipientKeyKey   = "network_fee_recipient_key"
)

func BindFlags(cmd *cobra.Command) error {
	cmd.PersistentFlags().String(configFlagKey, ".", "path to config file dir")

	cmd.PersistentFlags().String(cashInPeriodKey, ".", "Cash-in period.")
	cmd.PersistentFlags().String(caminoNodeHostKey, ".", "Camino node host.")
	cmd.PersistentFlags().String(httPortKey, ".", "App-service http port.")
	cmd.PersistentFlags().String(matrixAccessTokenKey, ".", "Access token that matrix will use in requests to app-service.")
	cmd.PersistentFlags().String(logLevelKey, ".", "Log level.")
	cmd.PersistentFlags().String(dbPathKey, ".", "Path to database.")
	cmd.PersistentFlags().String(dbNameKey, ".", "Database name.")
	cmd.PersistentFlags().String(migrationsPathKey, ".", "Path to database migrations folder.")
	cmd.PersistentFlags().String(cmAccountContractAddressKey, ".", "CM account contract address.")
	cmd.PersistentFlags().String(networkFeeRecipientKeyKey, ".", "Network fee recipient key.")

	return viper.BindPFlags(cmd.PersistentFlags())
}

type UnparsedConfig struct {
	CashInPeriod               time.Duration `mapstructure:"cash_in_period"`
	CaminoNodeHost             string        `mapstructure:"camino_node_host"`
	HTTPPort                   string        `mapstructure:"http_port"`
	MatrixAccessToken          string        `mapstructure:"matrix_access_token"`
	LogLevel                   string        `mapstructure:"log_level"`
	DBPath                     string        `mapstructure:"db_path"`
	DBName                     string        `mapstructure:"db_name"`
	MigrationsPath             string        `mapstructure:"migrations_path"`
	CMAccountContractAddress   string        `mapstructure:"cm_account_contract_address"`
	NetworkFeeRecipientKey     string        `mapstructure:"network_fee_recipient_key"`
	MinDurationUntilExpiration uint64        `mapstructure:"min_duration_until_expiration"`
}

type Config struct {
	CashInPeriod               time.Duration
	CaminoNodeHost             url.URL
	HTTPPort                   string
	MatrixAccessToken          string
	LogLevel                   string
	DBPath                     string
	DBName                     string
	MigrationsPath             string
	CMAccountContractAddress   common.Address
	NetworkFeeRecipientKey     *ecdsa.PrivateKey
	MinDurationUntilExpiration uint64
}

func ReadConfig(ctx context.Context, logger *zap.SugaredLogger) (*Config, error) {
	cr := &configReader{
		logger: logger,
	}

	return cr.readConfig(ctx)
}

type configReader struct {
	logger *zap.SugaredLogger
}

func (cr *configReader) readConfig(_ context.Context) (*Config, error) {
	configPath := viper.GetString(configFlagKey)
	if configPath == "" {
		err := errors.New("config path is empty")
		cr.logger.Error(err)
		return nil, err
	}

	viper.SetConfigName(configFileName)
	viper.AddConfigPath(".")
	viper.AddConfigPath(configPath)

	viper.SetEnvPrefix("CAMINO_APPSERVICE")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		var viperErr *viper.ConfigFileNotFoundError
		if ok := errors.As(err, viperErr); ok {
			cr.logger.Info("Config file not found")
		} else {
			cr.logger.Errorf("Error reading config file: %s", err)
		}
	}

	cfg := &UnparsedConfig{}
	if err := viper.Unmarshal(cfg); err != nil {
		cr.logger.Error(err)
		return nil, err
	}

	return cr.parseConfig(cfg)
}

func (cr *configReader) parseConfig(cfg *UnparsedConfig) (*Config, error) {
	networkFeeRecipientECDSAKey, err := crypto.HexToECDSA(cfg.NetworkFeeRecipientKey)
	if err != nil {
		cr.logger.Error(err)
		return nil, err
	}

	nodeURL, err := url.Parse(cfg.CaminoNodeHost)
	if err != nil {
		cr.logger.Errorf("Error parsing CaminoNodeHost URL: %s", err)
		return nil, err
	}

	return &Config{
		CashInPeriod:               cfg.CashInPeriod,
		CaminoNodeHost:             *nodeURL,
		HTTPPort:                   cfg.HTTPPort,
		MatrixAccessToken:          cfg.MatrixAccessToken,
		LogLevel:                   cfg.LogLevel,
		DBPath:                     cfg.DBPath,
		DBName:                     cfg.DBName,
		MigrationsPath:             cfg.MigrationsPath,
		CMAccountContractAddress:   common.HexToAddress(cfg.CMAccountContractAddress),
		NetworkFeeRecipientKey:     networkFeeRecipientECDSAKey,
		MinDurationUntilExpiration: cfg.MinDurationUntilExpiration,
	}, nil
}
