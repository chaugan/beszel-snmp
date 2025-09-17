package snmpmonitor

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
)

// Poller handles SNMP polling for a device
type Poller struct {
	device     DeviceConfig
	hubClient  *HubClient
	stopChan   chan struct{}
	mu         sync.RWMutex
	running    bool
	lastValues map[string]float64
}

// NewPoller creates a new poller for a device
func NewPoller(device DeviceConfig, hubClient *HubClient) (*Poller, error) {
	return &Poller{
		device:     device,
		hubClient:  hubClient,
		stopChan:   make(chan struct{}),
		lastValues: make(map[string]float64),
	}, nil
}

// Start starts the polling loop
func (p *Poller) Start(ctx context.Context) {
	p.mu.Lock()
	p.running = true
	p.mu.Unlock()

	ticker := time.NewTicker(p.device.GetPollInterval())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.Stop()
			return
		case <-p.stopChan:
			return
		case <-ticker.C:
			p.poll()
		}
	}
}

// Stop stops the polling loop
func (p *Poller) Stop() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		p.running = false
		close(p.stopChan)
	}
}

// poll performs a single SNMP poll
func (p *Poller) poll() {
	params := &gosnmp.GoSNMP{
		Target:    p.device.IP,
		Port:      161,
		Community: p.device.Community,
		Version:   gosnmp.Version2c,
		Timeout:   5 * time.Second,
		Retries:   1,
	}

	if err := params.Connect(); err != nil {
		log.Printf("Failed to connect to %s: %v", p.device.IP, err)
		return
	}
	defer params.Conn.Close()

	// Collect OIDs to poll
	var oids []string
	for _, metric := range p.device.Metrics {
		oids = append(oids, metric.OID)
	}

	if len(oids) == 0 {
		return
	}

	// Perform SNMP GET request
	result, err := params.Get(oids)
	if err != nil {
		log.Printf("SNMP GET failed for %s: %v", p.device.IP, err)
		return
	}

	// Process results
	metrics := make(map[string]MetricValue)
	for _, variable := range result.Variables {
		oid := variable.Name

		// Find the metric config for this OID
		var metricConfig MetricConfig
		var metricName string
		found := false

		for name, metric := range p.device.Metrics {
			if metric.OID == oid {
				metricConfig = metric
				metricName = name
				found = true
				break
			}
		}

		if !found {
			continue
		}

		// Convert value to float64
		value := p.convertSNMPValue(variable.Value)
		if value == nil {
			continue
		}

		// Apply scaling
		scaledValue := *value * metricConfig.Scale
		if metricConfig.Scale == 0 {
			scaledValue = *value
		}

		// Store the value
		p.mu.Lock()
		p.lastValues[metricName] = scaledValue
		p.mu.Unlock()

		// Create metric value for hub
		metrics[metricName] = MetricValue{
			Name:     metricConfig.Name,
			Value:    scaledValue,
			Unit:     metricConfig.Unit,
			Category: metricConfig.Category,
		}
	}

	// Send metrics to hub
	if len(metrics) > 0 {
		deviceData := DeviceData{
			Name:    p.device.Name,
			IP:      p.device.IP,
			Metrics: metrics,
		}

		// Use NotifyDevice to create per-device connections
		p.hubClient.NotifyDevice(deviceData)
	}
}

// convertSNMPValue converts SNMP value to float64
func (p *Poller) convertSNMPValue(value interface{}) *float64 {
	switch v := value.(type) {
	case int:
		f := float64(v)
		return &f
	case int32:
		f := float64(v)
		return &f
	case int64:
		f := float64(v)
		return &f
	case uint:
		f := float64(v)
		return &f
	case uint32:
		f := float64(v)
		return &f
	case uint64:
		f := float64(v)
		return &f
	case float32:
		f := float64(v)
		return &f
	case float64:
		return &v
	default:
		return nil
	}
}

// GetLastValues returns the last polled values
func (p *Poller) GetLastValues() map[string]float64 {
	p.mu.RLock()
	defer p.mu.RUnlock()

	result := make(map[string]float64)
	for k, v := range p.lastValues {
		result[k] = v
	}
	return result
}
