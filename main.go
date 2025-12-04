package main

import (
	"flag"
	"log"

	"web-server/internal/config"
	"web-server/internal/server"
)

var (
	configFile = flag.String("config", "default.conf", "Path to configuration file")
)

func main() {
	flag.Parse()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	srv := server.NewServer(cfg)
	if err := srv.Start(); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
