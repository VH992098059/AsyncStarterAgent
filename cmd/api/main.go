package main

import (
	"fmt"
	"os"

	"github.com/asyncstarter/agent/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config load failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("AsyncStarterAgent API starting on :%s (env=%s)\n", cfg.Port, cfg.Env)
}
