// Copyright (C) 2022-2025, Chain4Travel AG. All rights reserved.
// See the file LICENSE for licensing terms.

package config

import (
	"github.com/spf13/pflag"
)

const flagKeyConfig = "config"

func Flags() *pflag.FlagSet {
	flags := pflag.NewFlagSet("config", pflag.ExitOnError)

	flags.String(flagKeyConfig, ".", "path to config file dir")

	flags.String("cash_in_period", ".", "Cash-in period.")
	flags.String("c_chain_rpc_url", ".", "Camino c-chain rpc url.")
	flags.String("http_port", ".", "App-service http port.")
	flags.String("matrix_access_token", ".", "Access token that matrix will use in requests to app-service.")
	flags.String("log_level", ".", "Log level.")
	flags.String("db_path", ".", "Path to database.")
	flags.String("db_name", ".", "Database name.")
	flags.String("migrations_path", ".", "Path to database migrations folder.")
	flags.String("cm_account_contract_address", ".", "CM account contract address.")
	flags.String("network_fee_recipient_key", ".", "Network fee recipient key.")

	return flags
}
