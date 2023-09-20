package main

import (
	"log"

	"github.com/chain4travel/camino-synapse-app-service/cmd"
)

// ! appservice will not process events that happend while app-service wasn't running
// ! need to implement stuff with /sync for that
// ! upd: not sure, may be synapse will retry to very end of it

// ! if app-service will response to synapse with error for some reason (e.g. db error on saving cheques)
// ! synapse server will schedule retry request, but increasing delay.
// ! and NO FUTHER event will be sent until app-service will response without error

func main() {
	if err := cmd.Execute(); err != nil {
		log.Fatalln(err)
	}
}
