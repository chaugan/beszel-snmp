package snmpagent

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/henrygd/beszel/internal/entities/system"
)

type deviceState struct {
	ip        net.IP
	finger    string
	hostname  string
	lastTemps map[string]float64
	// additional categories
	lastHumidity map[string]float64
	lastCO2      map[string]float64
	lastPressure map[string]float64
	lastPM25     map[string]float64
	lastPM10     map[string]float64
	lastVOC      map[string]float64
	lastSend  time.Time
	mu        sync.Mutex
}

func newDeviceState(ip net.IP, finger, hostname string) *deviceState {
	return &deviceState{
		ip:           ip,
		finger:       finger,
		hostname:     hostname,
		lastTemps:    map[string]float64{},
		lastHumidity: map[string]float64{},
		lastCO2:      map[string]float64{},
		lastPressure: map[string]float64{},
		lastPM25:     map[string]float64{},
		lastPM10:     map[string]float64{},
		lastVOC:      map[string]float64{},
	}
}

func deriveFingerprint(ip net.IP, sysName string, tmpl string) string {
	base := ip.String()
	if sysName != "" {
		base = sysName + "-" + base
	}
	if tmpl != "" {
		base = replacePlaceholders(tmpl, ip, sysName)
	}
	sum := sha256.Sum256([]byte(base))
	return hex.EncodeToString(sum[:24])
}

func replacePlaceholders(tmpl string, ip net.IP, sysName string) string {
	repl := tmpl
	repl = strings.ReplaceAll(repl, "%IP%", ip.String())
	repl = strings.ReplaceAll(repl, "%sysName%", sysName)
	return repl
}

func (ds *deviceState) setMetric(category, name string, val float64) {
	switch strings.ToLower(category) {
	case "temperature", "temp", "t":
		ds.lastTemps[name] = val
	case "humidity", "hum", "h":
		ds.lastHumidity[name] = val
	case "co2":
		ds.lastCO2[name] = val
	case "pressure", "press", "pr":
		ds.lastPressure[name] = val
	case "pm25":
		ds.lastPM25[name] = val
	case "pm10":
		ds.lastPM10[name] = val
	case "voc":
		ds.lastVOC[name] = val
	default:
		// ignore unknown categories for now
	}
}

func (ds *deviceState) buildCombinedData(agentVersion string) *system.CombinedData {
	// Only sensors for now
	stats := system.Stats{
		Temperatures: map[string]float64{},
	}
	for k, v := range ds.lastTemps {
		stats.Temperatures[k] = v
	}
	// add other categories if present
	if len(ds.lastHumidity) > 0 {
		stats.Humidity = map[string]float64{}
		for k, v := range ds.lastHumidity {
			stats.Humidity[k] = v
		}
	}
	if len(ds.lastCO2) > 0 {
		stats.CO2 = map[string]float64{}
		for k, v := range ds.lastCO2 {
			stats.CO2[k] = v
		}
	}
	if len(ds.lastPressure) > 0 {
		stats.Pressure = map[string]float64{}
		for k, v := range ds.lastPressure {
			stats.Pressure[k] = v
		}
	}
	if len(ds.lastPM25) > 0 {
		stats.PM25 = map[string]float64{}
		for k, v := range ds.lastPM25 {
			stats.PM25[k] = v
		}
	}
	if len(ds.lastPM10) > 0 {
		stats.PM10 = map[string]float64{}
		for k, v := range ds.lastPM10 {
			stats.PM10[k] = v
		}
	}
	if len(ds.lastVOC) > 0 {
		stats.VOC = map[string]float64{}
		for k, v := range ds.lastVOC {
			stats.VOC[k] = v
		}
	}
	// Build info with dashboard summaries for SNMP
	info := system.Info{Hostname: ds.hostname, AgentVersion: agentVersion, AgentType: "snmp"}
	if len(ds.lastTemps) > 0 {
		info.DashboardTemp = maxMap(ds.lastTemps)
	}
	if len(ds.lastHumidity) > 0 {
		info.DashboardHumidity = maxMap(ds.lastHumidity)
	}
	if len(ds.lastCO2) > 0 {
		info.DashboardCO2 = maxMap(ds.lastCO2)
	}
	if len(ds.lastPressure) > 0 {
		// pressure tends to be similar across sensors; average is fine
		info.DashboardPressure = avgMap(ds.lastPressure)
	}
	if len(ds.lastPM25) > 0 {
		info.DashboardPM25 = maxMap(ds.lastPM25)
	}
	if len(ds.lastPM10) > 0 {
		info.DashboardPM10 = maxMap(ds.lastPM10)
	}
	if len(ds.lastVOC) > 0 {
		info.DashboardVOC = maxMap(ds.lastVOC)
	}
	return &system.CombinedData{Stats: stats, Info: info}
}

func maxMap(m map[string]float64) float64 {
	var max float64
	var set bool
	for _, v := range m {
		if !set || v > max {
			max = v
			set = true
		}
	}
	return max
}

func avgMap(m map[string]float64) float64 {
	var sum float64
	var n int
	for _, v := range m {
		sum += v
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}
