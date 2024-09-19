package main

import (
	"log"

	"github.com/chain4travel/camino-synapse-app-service/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
