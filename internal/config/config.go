package config

import (
	"context"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	configFileName = "camino-synapse-appservice"

	configFlagKey = "config"

	cashOutPeriodKey        = "cashout_period"
	cashOutTxCheckPeriodKey = "cashout_tx_check_period"
	caminoNodeHostKey       = "camino_node_host"
	matrixHostKey           = "matrix_host"
	httPortKey              = "http_port"
	accessTokenKey          = "access_token"
	matrixAccessTokenKey    = "matrix_access_token"
	logLevelKey             = "log_level"
	dbPathKey               = "db_path"
)

func BindFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String(configFlagKey, ".", "path to config file dir")

	cmd.PersistentFlags().String(cashOutPeriodKey, ".", "cashout_period")
	cmd.PersistentFlags().String(cashOutTxCheckPeriodKey, ".", "cashout_tx_check_period")
	cmd.PersistentFlags().String(caminoNodeHostKey, ".", "camino_node_host")
	cmd.PersistentFlags().String(matrixHostKey, ".", "matrix_host")
	cmd.PersistentFlags().String(httPortKey, ".", "http_port")
	cmd.PersistentFlags().String(accessTokenKey, ".", "access_token")
	cmd.PersistentFlags().String(matrixAccessTokenKey, ".", "matrix_access_token")
	cmd.PersistentFlags().String(logLevelKey, ".", "log_level")
	cmd.PersistentFlags().String(dbPathKey, ".", "db_path")

	viper.BindPFlag(configFlagKey, cmd.PersistentFlags().Lookup(configFlagKey))

	viper.BindPFlag(cashOutPeriodKey, cmd.PersistentFlags().Lookup(cashOutPeriodKey))
	viper.BindPFlag(cashOutTxCheckPeriodKey, cmd.PersistentFlags().Lookup(cashOutTxCheckPeriodKey))
	viper.BindPFlag(caminoNodeHostKey, cmd.PersistentFlags().Lookup(caminoNodeHostKey))
	viper.BindPFlag(matrixHostKey, cmd.PersistentFlags().Lookup(matrixHostKey))
	viper.BindPFlag(httPortKey, cmd.PersistentFlags().Lookup(httPortKey))
	viper.BindPFlag(accessTokenKey, cmd.PersistentFlags().Lookup(accessTokenKey))
	viper.BindPFlag(matrixAccessTokenKey, cmd.PersistentFlags().Lookup(matrixAccessTokenKey))
	viper.BindPFlag(logLevelKey, cmd.PersistentFlags().Lookup(logLevelKey))
	viper.BindPFlag(dbPathKey, cmd.PersistentFlags().Lookup(dbPathKey))
}

type Config struct {
	CashOutPeriod        time.Duration `mapstructure:"cashout_period"`
	CashOutTxCheckPeriod time.Duration `mapstructure:"cashout_tx_check_period"`
	CaminoNodeHost       string        `mapstructure:"camino_node_host"`
	MatrixHost           string        `mapstructure:"matrix_host"`
	HTTPPort             string        `mapstructure:"http_port"`
	AccessToken          string        `mapstructure:"access_token"`
	MatrixAccessToken    string        `mapstructure:"matrix_access_token"`
	LogLevel             string        `mapstructure:"log_level"`
	DBPath               string        `mapstructure:"db_path"`
}

func ReadConfig(ctx context.Context, logger *zap.SugaredLogger) (*Config, error) {
	logger.Debug("Reading config...")
	viper.SetConfigName(configFileName)
	viper.AddConfigPath(".")
	viper.AddConfigPath(viper.GetString(configFlagKey)) // must already be bound from flag

	if err := viper.ReadInConfig(); err != nil {
		logger.Info(err)
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		logger.Error(err)
		return nil, err
	}

	return cfg, nil
}
