package main

import (
	"flag"
	"os"

	log "github.com/sirupsen/logrus"
)

func main() {
	port := flag.Int("port", 0, "The port your server exposes to clients")
	debugMode := flag.Bool("debug", false, "Print extra info")
	flag.Parse()

	if *debugMode {
		log.Info("Debug mode active")
		log.SetLevel(log.DebugLevel)
	}

	server, err := newServer(*port)
	if err != nil {
		log.Errorf("Error setting up server: %v\n", err)
		os.Exit(1)
	}
	server.runServer()
}
