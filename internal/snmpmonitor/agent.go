package snmpmonitor

import (
	"context"
	"log"
	"sync"
)

// Agent represents the SNMP monitor
type Agent struct {
	config    *Config
	hubConfig *HubConfig
	webServer *WebServer
	pollers   map[string]*Poller
	hubClient *HubClient
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// NewAgent creates a new SNMP monitor
func NewAgent(configPath string) (*Agent, error) {
	config, hubConfig, webServerConfig, err := LoadConfig(configPath)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	agent := &Agent{
		config:    config,
		hubConfig: hubConfig,
		pollers:   make(map[string]*Poller),
		ctx:       ctx,
		cancel:    cancel,
	}

	// Initialize web server
	agent.webServer, err = NewWebServer(agent, webServerConfig)
	if err != nil {
		cancel()
		return nil, err
	}

	// Initialize hub client
	agent.hubClient, err = NewHubClient(*hubConfig)
	if err != nil {
		cancel()
		return nil, err
	}

	return agent, nil
}

// Run starts the SNMP monitor
func (a *Agent) Run() error {
	// Start web server
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		log.Printf("Starting web server on port %d", a.webServer.config.Port)
		if err := a.webServer.Start(); err != nil {
			log.Printf("Web server error: %v", err)
		}
	}()

	// Start pollers for each device
	for _, device := range a.config.Devices {
		poller, err := NewPoller(device, a.hubClient)
		if err != nil {
			log.Printf("Failed to create poller for device %s: %v", device.Name, err)
			continue
		}

		a.pollers[device.Name] = poller
		a.wg.Add(1)
		go func(p *Poller) {
			defer a.wg.Done()
			log.Printf("Starting poller for device %s", p.device.Name)
			p.Start(a.ctx)
		}(poller)
	}

	// Wait for context cancellation
	<-a.ctx.Done()
	log.Println("Shutting down container agent...")

	// Wait for all goroutines to finish
	a.wg.Wait()
	return nil
}

// Stop stops the container agent
func (a *Agent) Stop() {
	a.cancel()
}

// GetConfig returns the current configuration
func (a *Agent) GetConfig() *Config {
	return a.config
}

// GetPollerStatus returns the status and metrics for a specific device
func (a *Agent) GetPollerStatus(deviceName string) (string, map[string]float64) {
	if poller, exists := a.pollers[deviceName]; exists {
		return poller.GetStatus(), poller.GetLastValues()
	}
	return "Not Found", make(map[string]float64)
}

// GetHubConfig returns the hub configuration
func (a *Agent) GetHubConfig() *HubConfig {
	return a.hubConfig
}

// GetWebServerConfig returns the web server configuration
func (a *Agent) GetWebServerConfig() *WebServerConfig {
	return a.webServer.config
}

// UpdateConfig updates the configuration and restarts pollers and hub client
func (a *Agent) UpdateConfig(newConfig *Config) error {
	a.config = newConfig

	// Check if hub config changed
	hubConfigChanged := false
	if newConfig.Hub != nil && (newConfig.Hub.URL != "" || newConfig.Hub.Token != "" || newConfig.Hub.Key != "") {
		// Use web interface config
		oldURL := a.hubConfig.URL
		oldToken := a.hubConfig.Token
		oldKey := a.hubConfig.Key

		a.hubConfig.URL = newConfig.Hub.URL
		a.hubConfig.Token = newConfig.Hub.Token
		a.hubConfig.Key = newConfig.Hub.Key

		// Check if any hub setting changed
		if oldURL != a.hubConfig.URL || oldToken != a.hubConfig.Token || oldKey != a.hubConfig.Key {
			hubConfigChanged = true
			log.Println("Hub configuration changed, will restart hub client")
		}
	}

	// Restart hub client if config changed
	if hubConfigChanged {
		var err error
		a.hubClient, err = NewHubClient(*a.hubConfig)
		if err != nil {
			log.Printf("Failed to create new hub client: %v", err)
			return err
		}
		log.Println("Hub client restarted with new configuration")
	}

	// Stop existing pollers
	for name, poller := range a.pollers {
		poller.Stop()
		delete(a.pollers, name)
	}

	// Start new pollers
	for _, device := range newConfig.Devices {
		poller, err := NewPoller(device, a.hubClient)
		if err != nil {
			log.Printf("Failed to create poller for device %s: %v", device.Name, err)
			continue
		}

		a.pollers[device.Name] = poller
		go func(p *Poller) {
			log.Printf("Starting poller for device %s", p.device.Name)
			p.Start(a.ctx)
		}(poller)
	}

	return nil
}
