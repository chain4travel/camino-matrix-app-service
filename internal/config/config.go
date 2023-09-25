package config

import (
	"context"
	"time"

	"github.com/ava-labs/avalanchego/utils/wrappers"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

const (
	configFileName = "camino-synapse-app-service"

	configFlagKey = "config"

	cashOutPeriodKey     = "cashout_period"
	caminoNodeHostKey    = "camino_node_host"
	matrixHostKey        = "matrix_host"
	httPortKey           = "http_port"
	accessTokenKey       = "access_token"
	matrixAccessTokenKey = "matrix_access_token"
	logLevelKey          = "log_level"
	dbPathKey            = "db_path"
	dbNameKey            = "db_name"
	migrationsPathKey    = "migrations_path"
)

func BindFlags(cmd *cobra.Command) error {
	cmd.PersistentFlags().String(configFlagKey, ".", "path to config file dir")

	cmd.PersistentFlags().String(cashOutPeriodKey, ".", "Cashout period.")
	cmd.PersistentFlags().String(caminoNodeHostKey, ".", "Camino node host.")
	cmd.PersistentFlags().String(matrixHostKey, ".", "matrix host.")
	cmd.PersistentFlags().String(httPortKey, ".", "App-service http port.")
	cmd.PersistentFlags().String(accessTokenKey, ".", "Access token authorized for app-service by matrix server.")
	cmd.PersistentFlags().String(matrixAccessTokenKey, ".", "Access token that matrix will use in requests to app-service.")
	cmd.PersistentFlags().String(logLevelKey, ".", "Log level.")
	cmd.PersistentFlags().String(dbPathKey, ".", "Path to database.")
	cmd.PersistentFlags().String(dbNameKey, ".", "Database name.")
	cmd.PersistentFlags().String(migrationsPathKey, ".", "Path to database migrations folder.")

	errs := wrappers.Errs{}
	errs.Add(
		viper.BindPFlag(configFlagKey, cmd.PersistentFlags().Lookup(configFlagKey)),

		viper.BindPFlag(cashOutPeriodKey, cmd.PersistentFlags().Lookup(cashOutPeriodKey)),
		viper.BindPFlag(caminoNodeHostKey, cmd.PersistentFlags().Lookup(caminoNodeHostKey)),
		viper.BindPFlag(matrixHostKey, cmd.PersistentFlags().Lookup(matrixHostKey)),
		viper.BindPFlag(httPortKey, cmd.PersistentFlags().Lookup(httPortKey)),
		viper.BindPFlag(accessTokenKey, cmd.PersistentFlags().Lookup(accessTokenKey)),
		viper.BindPFlag(matrixAccessTokenKey, cmd.PersistentFlags().Lookup(matrixAccessTokenKey)),
		viper.BindPFlag(logLevelKey, cmd.PersistentFlags().Lookup(logLevelKey)),
		viper.BindPFlag(dbPathKey, cmd.PersistentFlags().Lookup(dbPathKey)),
		viper.BindPFlag(dbNameKey, cmd.PersistentFlags().Lookup(dbNameKey)),
		viper.BindPFlag(migrationsPathKey, cmd.PersistentFlags().Lookup(migrationsPathKey)),
	)
	return errs.Err
}

type Config struct {
	CashOutPeriod     time.Duration `mapstructure:"cashout_period"`
	CaminoNodeHost    string        `mapstructure:"camino_node_host"`
	MatrixHost        string        `mapstructure:"matrix_host"`
	HTTPPort          string        `mapstructure:"http_port"`
	AccessToken       string        `mapstructure:"access_token"`
	MatrixAccessToken string        `mapstructure:"matrix_access_token"`
	LogLevel          string        `mapstructure:"log_level"`
	DBPath            string        `mapstructure:"db_path"`
	DBName            string        `mapstructure:"db_name"`
	MigrationsPath    string        `mapstructure:"migrations_path"`
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
