package snmpmonitor

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config represents the configuration for the SNMP monitor
type Config struct {
	Hub       *HubConfig       `json:"hub,omitempty"`
	WebServer *WebServerConfig `json:"web_server,omitempty"`
	Devices   []DeviceConfig   `json:"devices"`
}

// HubConfig defines the hub connection settings
type HubConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	Key   string `json:"key"`
}

// WebServerConfig defines the web server settings
type WebServerConfig struct {
	Port int `json:"port"`
}

// DeviceConfig defines a device to monitor
type DeviceConfig struct {
	Name         string                  `json:"name"`
	IP           string                  `json:"ip"`
	Community    string                  `json:"community"`
	PollInterval int                     `json:"poll_interval_sec"` // in seconds
	Metrics      map[string]MetricConfig `json:"metrics"`
}

// MetricConfig defines how to poll and interpret an OID
type MetricConfig struct {
	OID      string  `json:"oid"`
	Name     string  `json:"name"`
	Unit     string  `json:"unit"`
	Category string  `json:"category"`
	Scale    float64 `json:"scale"`
}

// DeviceData represents data to send to the hub
type DeviceData struct {
	Name    string                 `json:"name"`
	IP      string                 `json:"ip"`
	Metrics map[string]MetricValue `json:"metrics"`
}

// MetricValue represents a metric value
type MetricValue struct {
	Name     string  `json:"name"`
	Value    float64 `json:"value"`
	Unit     string  `json:"unit"`
	Category string  `json:"category"`
}

// LoadConfig loads the configuration from a JSON file and environment variables
func LoadConfig(path string) (*Config, *HubConfig, *WebServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Load hub config - use web config if available, otherwise fall back to environment variables
	hubConfig := &HubConfig{}
	if config.Hub != nil && (config.Hub.URL != "" || config.Hub.Token != "" || config.Hub.Key != "") {
		// Use web interface config
		hubConfig = config.Hub
	} else {
		// Fall back to environment variables
		hubConfig = &HubConfig{
			URL:   os.Getenv("BESZEL_HUB_URL"),
			Token: os.Getenv("BESZEL_HUB_TOKEN"),
			Key:   os.Getenv("BESZEL_HUB_KEY"),
		}
	}

	// Load web server config - use web config if available, otherwise fall back to environment variables
	webServerConfig := &WebServerConfig{}
	if config.WebServer != nil && config.WebServer.Port > 0 {
		// Use web interface config
		webServerConfig = config.WebServer
	} else {
		// Fall back to environment variables
		webServerConfig = &WebServerConfig{
			Port: 6655, // Default port
		}
		if portStr := os.Getenv("BESZEL_WEB_PORT"); portStr != "" {
			if port, err := strconv.Atoi(portStr); err == nil && port > 0 {
				webServerConfig.Port = port
			}
		}
	}

	return &config, hubConfig, webServerConfig, nil
}

// SaveConfig saves the configuration to a JSON file
func (c *Config) SaveConfig(path string) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// GetPollInterval returns the poll interval for a device
func (d *DeviceConfig) GetPollInterval() time.Duration {
	if d.PollInterval <= 0 {
		return 30 * time.Second // default 30 seconds
	}
	return time.Duration(d.PollInterval) * time.Second
}
