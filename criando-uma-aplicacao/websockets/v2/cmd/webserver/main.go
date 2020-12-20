package main

import (
	"log"
	"net/http"
	"os"

	poquer "github.com/larien/learn-go-with-tests/criando-uma-aplicacao/websockets/v2"
)

const dbFileName = "partida.db.json"

func main() {
	db, err := os.OpenFile(dbFileName, os.O_RDWR|os.O_CREATE, 0666)

	if err != nil {
		log.Fatalf("problem opening %s %v", dbFileName, err)
	}

	armazenamento, err := poquer.NewFileSystemPlayerStore(db)

	if err != nil {
		log.Fatalf("problem creating file system player armazenamento, %v ", err)
	}

	partida := poquer.NovoTexasHoldem(poquer.AlertadorDeBlindFunc(poquer.Alerter), armazenamento)

	server, err := poquer.NewPlayerServer(armazenamento, partida)

	if err != nil {
		log.Fatalf("problem creating player server %v", err)
	}

	if err := http.ListenAndServe(":5000", server); err != nil {
		log.Fatalf("could not listen on port 5000 %v", err)
	}
}
