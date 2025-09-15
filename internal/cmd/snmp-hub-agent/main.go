package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/henrygd/beszel/internal/snmpagent"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: snmp-hub-agent <devices.json>")
	}
	devicesPath := os.Args[1]
	
	// Load devices configuration
	devices, err := loadDevicesConfig(devicesPath)
	if err != nil {
		log.Fatal(err)
	}
	
	// Create config from environment variables and devices
	cfg, err := createConfigFromEnv(devices)
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

// loadDevicesConfig loads only the devices array from the JSON file
func loadDevicesConfig(path string) ([]snmpagent.DeviceConfig, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	var config struct {
		Devices []snmpagent.DeviceConfig `json:"devices"`
	}
	
	if err := json.Unmarshal(b, &config); err != nil {
		return nil, fmt.Errorf("invalid devices config: %w", err)
	}
	
	return config.Devices, nil
}

// createConfigFromEnv creates a full config using environment variables and devices
func createConfigFromEnv(devices []snmpagent.DeviceConfig) (*snmpagent.Config, error) {
	cfg := &snmpagent.Config{
		Devices: devices,
	}
	
	// Hub configuration from environment
	cfg.Hub.URL = getEnv("BESZEL_HUB_URL", "")
	cfg.Hub.Token = getEnv("BESZEL_HUB_TOKEN", "")
	cfg.Hub.Key = getEnv("BESZEL_HUB_KEY", "")
	
	// Defaults configuration from environment
	cfg.Defaults.SendIntervalSec = getEnvInt("BESZEL_SEND_INTERVAL_SEC", 10)
	cfg.Defaults.PollIntervalSec = getEnvInt("BESZEL_POLL_INTERVAL_SEC", 30)
	cfg.Defaults.ResolveMIBs = getEnvBool("BESZEL_RESOLVE_MIBS", false)
	cfg.Defaults.MIBPaths = getEnvStringSlice("BESZEL_MIB_PATHS", []string{"/usr/share/snmp/mibs"})
	cfg.Defaults.Round1 = getEnvBool("BESZEL_ROUND1", true)
	cfg.Defaults.LogUnknown = getEnvBool("BESZEL_LOG_UNKNOWN", true)
	cfg.Defaults.Communities = getEnvStringSlice("BESZEL_COMMUNITIES", []string{"public"})
	cfg.Defaults.ListenAddr = getEnv("BESZEL_LISTEN_ADDR", ":9162")
	
	// Validate required fields
	if cfg.Hub.URL == "" {
		return nil, fmt.Errorf("BESZEL_HUB_URL environment variable is required")
	}
	if cfg.Hub.Token == "" {
		return nil, fmt.Errorf("BESZEL_HUB_TOKEN environment variable is required")
	}
	if cfg.Hub.Key == "" {
		return nil, fmt.Errorf("BESZEL_HUB_KEY environment variable is required")
	}
	
	return cfg, nil
}

// Helper functions for environment variables
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvBool(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		if boolValue, err := strconv.ParseBool(value); err == nil {
			return boolValue
		}
	}
	return defaultValue
}

func getEnvStringSlice(key string, defaultValue []string) []string {
	if value := os.Getenv(key); value != "" {
		return strings.Split(value, ",")
	}
	return defaultValue
}
