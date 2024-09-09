package config

import (
	"context"
	"time"

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

	return viper.BindPFlags(cmd.PersistentFlags())
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
	viper.SetConfigName(configFileName)
	viper.AddConfigPath(".")
	viper.AddConfigPath(viper.GetString(configFlagKey)) // must already be bound from flag

	viper.SetEnvPrefix("CAMINO_APPSERVICE")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.Info("Config file not found")
		} else {
			logger.Errorf("Error reading config file: %s", err)
		}
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		logger.Error(err)
		return nil, err
	}

	return cfg, nil
}
