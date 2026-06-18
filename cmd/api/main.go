package main

import (
	"fmt"
	"log"
	"os"

	"github.com/asyncstarter/agent/internal/config"
	"github.com/asyncstarter/agent/internal/server"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config load failed: %v", err)
	}
	addr := ":" + cfg.Port
	fmt.Printf("AsyncStarterAgent API starting on %s (env=%s)\n", addr, cfg.Env)
	if err := server.New(cfg).Run(addr); err != nil {
		fmt.Fprintf(os.Stderr, "server failed: %v\n", err)
		os.Exit(1)
	}
}
