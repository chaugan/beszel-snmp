package main

import (
	"log"
	"os"

	"github.com/henrygd/beszel/internal/snmpagent"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: snmp-hub-agent <config.json>")
	}
	cfgPath := os.Args[1]
	cfg, err := snmpagent.LoadConfig(cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	a, err := snmpagent.NewAgent(cfg)
	if err != nil {
		log.Fatal(err)
	}
	if err := a.Run(); err != nil {
		log.Fatal(err)
	}
}
