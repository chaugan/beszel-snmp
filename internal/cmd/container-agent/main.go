package main

import (
	"log"
	"os"

	"github.com/henrygd/beszel/internal/containeragent"
)

func main() {
	// Get config file path from environment or use default
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/etc/beszel/container-agent.json"
	}

	agent, err := containeragent.NewAgent(configPath)
	if err != nil {
		log.Fatal("Failed to create container agent:", err)
	}

	log.Println("Starting container agent...")
	if err := agent.Run(); err != nil {
		log.Fatal("Container agent failed:", err)
	}
}
