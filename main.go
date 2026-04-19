package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/mostlygeek/llama-swap/proxy"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	var (
		configFile  = flag.String("config", "config.yaml", "path to configuration file")
		listenAddr  = flag.String("listen", ":11434", "address to listen on") // use ollama's default port
		showVersion = flag.Bool("version", false, "print version information and exit")
		logLevel    = flag.String("log-level", "info", "log level (debug, info, warn, error)")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("llama-swap %s (%s) built %s\n", version, commit, date)
		os.Exit(0)
	}

	// Validate config file exists
	if _, err := os.Stat(*configFile); os.IsNotExist(err) {
		log.Fatalf("config file not found: %s", *configFile)
	}

	log.Printf("llama-swap %s starting", version)
	log.Printf("loading config from: %s", *configFile)

	cfg, err := proxy.LoadConfig(*configFile)
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	server, err := proxy.NewServer(cfg, *listenAddr, *logLevel)
	if err != nil {
		log.Fatalf("failed to create server: %v", err)
	}

	log.Printf("listening on %s", *listenAddr)
	if err := server.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
