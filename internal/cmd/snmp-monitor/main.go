package main

import (
	"log"
	"os"

	"github.com/henrygd/beszel/internal/snmpmonitor"
)

func main() {
	// Get config file path from environment or use default
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/etc/beszel/snmp-monitor.json"
	}

	agent, err := snmpmonitor.NewAgent(configPath)
	if err != nil {
		log.Fatal("Failed to create container agent:", err)
	}

	log.Println("Starting SNMP monitor...")
	if err := agent.Run(); err != nil {
		log.Fatal("SNMP monitor failed:", err)
	}
}
