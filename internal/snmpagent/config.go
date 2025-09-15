package snmpagent

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"regexp"
	"time"
)

type HubConfig struct {
	URL   string `json:"url"`
	Token string `json:"token"`
	Key   string `json:"key"`
}

type Defaults struct {
	SendIntervalSec int      `json:"send_interval_sec"`
	PollIntervalSec int      `json:"poll_interval_sec"`
	ResolveMIBs     bool     `json:"resolve_mibs"`
	MIBPaths        []string `json:"mib_paths"`
	Round1          bool     `json:"round1"`
	LogUnknown      bool     `json:"log_unknown"`
	Communities     []string `json:"communities"`
	ListenAddr      string   `json:"listen_addr"`
}

type OIDMap struct {
	Name     string  `json:"name"`
	Kind     string  `json:"kind"`
	Unit     string  `json:"unit"`
	Category string  `json:"category"`
	Scale    float64 `json:"scale"`
}

type DeviceMatch struct {
	IPRegex string `json:"ip_regex"`
}

type DeviceConfig struct {
	Match            DeviceMatch       `json:"match"`
	FingerprintTmpl  string            `json:"fingerprint"`
	HostnameTemplate string            `json:"hostname_template"`
	Poll             bool              `json:"poll"`
	PollIntervalSec  int               `json:"poll_interval_sec"`
	Communities      []string          `json:"communities"`
	Oids             map[string]OIDMap `json:"oids"`

	// compiled
	ipRegex *regexp.Regexp `json:"-"`
}

type Config struct {
	Hub      HubConfig      `json:"hub"`
	Defaults Defaults       `json:"defaults"`
	Devices  []DeviceConfig `json:"devices"`
}

func LoadConfig(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	// defaults
	if c.Defaults.SendIntervalSec == 0 {
		c.Defaults.SendIntervalSec = 10
	}
	if c.Defaults.PollIntervalSec == 0 {
		c.Defaults.PollIntervalSec = 30
	}
	if c.Defaults.ListenAddr == "" {
		c.Defaults.ListenAddr = ":9162"
	}
	if c.Defaults.Communities == nil {
		c.Defaults.Communities = []string{"public"}
	}
	for i := range c.Devices {
		if c.Devices[i].PollIntervalSec == 0 {
			c.Devices[i].PollIntervalSec = c.Defaults.PollIntervalSec
		}
		if len(c.Devices[i].Communities) == 0 {
			c.Devices[i].Communities = c.Defaults.Communities
		}
		if c.Devices[i].Match.IPRegex != "" {
			re, err := regexp.Compile(c.Devices[i].Match.IPRegex)
			if err != nil {
				return nil, fmt.Errorf("bad ip_regex for device %d: %w", i, err)
			}
			c.Devices[i].ipRegex = re
		}
	}
	return &c, nil
}

func (c *Config) SendInterval() time.Duration { return time.Duration(c.Defaults.SendIntervalSec) * time.Second }

// MatchDevice returns the first device config matching the provided IP address.
func (c *Config) MatchDevice(ip net.IP) *DeviceConfig {
	ipStr := ip.String()
	for i := range c.Devices {
		d := &c.Devices[i]
		if d.ipRegex == nil || d.ipRegex.MatchString(ipStr) {
			return d
		}
	}
	return nil
}
