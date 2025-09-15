package snmpagent

import (
	"log/slog"
	"math"
	"strings"
)

// shouldSend tells whether this OID should be forwarded to Beszel
func shouldSend(om OIDMap) bool {
	switch strings.ToLower(om.Category) {
	case "temperature", "temp", "t",
		"humidity", "hum", "h",
		"co2",
		"pressure", "press", "pr",
		"pm25",
		"pm10",
		"voc":
		return true
	default:
		return false
	}
}

func transformValue(scale float64, v float64, round1 bool) float64 {
	if scale == 0 {
		scale = 1
	}
	val := v / scale
	if round1 {
		val = math.Round(val*10) / 10
	}
	return val
}

func logUnknownOID(oid string, value any, enabled bool) {
	if enabled {
		slog.Info("unknown_oid", "oid", oid, "value", value)
	}
}
