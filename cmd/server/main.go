package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	_ "github.com/mattn/go-sqlite3"
	"github.com/zdgeier/jamsync/internal/server/server"
)

func main() {
	closer, err := server.New()
	if err != nil {
		log.Fatal(err)
	}
	log.Println("Jamsync server is running...")

	done := make(chan os.Signal, 1)
	signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
	<-done

	log.Println("Jamsync server is stopping...")

	closer()
}
