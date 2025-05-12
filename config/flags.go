// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"github.com/spf13/pflag"
)

const flagKeyConfig = "config"

func Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("config", pflag.ExitOnError)

	flags.String(flagKeyConfig, "camino-matrix-app-service.yaml", "path to config file dir")

	// Main config flags
	flags.String("log_level", ".", "Log level.")
	flags.String("chain_rpc_url", ".", "Camino chain rpc url.")
	flags.String("network_fee_recipient_cm_account_address", ".", "Network fee recipient CMAccount address.")
	flags.String("network_fee_recipient_bot_key", ".", "Network fee recipient bot key.")
	flags.Uint64("min_cheque_duration_until_expiration", 3600*24*30*6, "Minimum valid duration until cheque expiration (in seconds).")
	flags.Int64("cash_in_period", 3600*24, "Cash-in period (in seconds).")

	// DB config flags
	flags.String("db.path", "camino-matrix-app-service-db", "Path to database dir.")

	// Matrix config flags
	flags.Uint64("matrix.http_port", 9090, "App-service http port.")
	flags.String("matrix.access_token", ".", "Access token that matrix will use in requests to app-service.")

	return flags
}
