#!/bin/bash

set -euo pipefail

go install -v github.com/golangci/golangci-lint/cmd/golangci-lint@v1.60
golangci-lint run --config .golangci.yml
