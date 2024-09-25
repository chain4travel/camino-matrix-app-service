package main

import (
	"log"

	"github.com/chain4travel/camino-matrix-app-service/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
