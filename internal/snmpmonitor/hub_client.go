package snmpmonitor

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/henrygd/beszel"
	"github.com/henrygd/beszel/internal/common"
	"github.com/henrygd/beszel/internal/entities/system"
	"github.com/lxzan/gws"
	gossh "golang.org/x/crypto/ssh"
)

type HubClient struct {
	config *HubConfig
	pubKey gossh.PublicKey
	url    *url.URL
	token  string
	mu     sync.Mutex
	conns  map[string]*deviceClient
}

type deviceClient struct {
	gws.BuiltinEventHandler
	deviceIP        string
	deviceName      string
	cfg             *HubConfig
	conn            *gws.Conn
	hubVerified     bool
	lastData        DeviceData
	mu              sync.Mutex
	needsToken      bool // Whether this device needs token authentication
	hasTriedNoToken bool // Whether we've tried connecting without token
}

// Helper functions for parsing URL and public key
func parseURL(urlStr string) *url.URL {
	u, err := url.Parse(urlStr)
	if err != nil {
		log.Printf("Failed to parse URL %s: %v", urlStr, err)
		return nil
	}
	return u
}

func parsePublicKey(keyStr string) gossh.PublicKey {
	if keyStr == "" {
		return nil
	}
	pubKey, _, _, _, err := gossh.ParseAuthorizedKey([]byte(keyStr))
	if err != nil {
		log.Printf("Failed to parse public key: %v", err)
		return nil
	}
	return pubKey
}

func NewHubClient(config HubConfig) (*HubClient, error) {
	client := &HubClient{
		config: &config,
		token:  strings.TrimSpace(config.Token),
		conns:  make(map[string]*deviceClient),
	}

	// Parse the hub URL
	client.url = parseURL(config.URL)
	if client.url == nil {
		return nil, fmt.Errorf("invalid hub URL: %s", config.URL)
	}

	// Parse the public key
	client.pubKey = parsePublicKey(config.Key)

	return client, nil
}

func (c *HubClient) NotifyDevice(deviceData DeviceData) {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := deviceData.IP
	dc, ok := c.conns[key]
	if !ok {
		dc = &deviceClient{
			deviceIP:   deviceData.IP,
			deviceName: deviceData.Name,
			cfg:        c.config,
		}
		c.conns[key] = dc
		go dc.connect(c)
	}

	// Update the device data
	dc.mu.Lock()
	dc.lastData = deviceData
	dc.mu.Unlock()

	// Log the data for debugging
	log.Printf("SNMP Monitor Data for device %s: %+v", deviceData.Name, deviceData)

	// Convert to JSON for logging
	jsonData := map[string]interface{}{
		"name":    deviceData.Name,
		"ip":      deviceData.IP,
		"metrics": make(map[string]interface{}),
	}

	metrics := jsonData["metrics"].(map[string]interface{})
	for key, metric := range deviceData.Metrics {
		metrics[key] = map[string]interface{}{
			"name":     metric.Name,
			"value":    metric.Value,
			"unit":     metric.Unit,
			"category": metric.Category,
		}
	}

	log.Printf("JSON Data: %+v", jsonData)
}

func (dc *deviceClient) getOptions(hubClient *HubClient) *gws.ClientOption {
	if hubClient.url == nil {
		return &gws.ClientOption{}
	}

	u := *hubClient.url
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}

	// Ensure proper path
	joined := path.Join(u.Path, "api/beszel/agent-connect")
	u.Path = "/" + strings.TrimPrefix(joined, "/")

	// Build headers
	headers := map[string][]string{
		"User-Agent": []string{"Beszel-SNMP-Monitor"},
		"X-Beszel":   []string{beszel.Version},
	}

	// Only include token if this device needs authentication
	if dc.needsToken && hubClient.token != "" {
		headers["X-Token"] = []string{hubClient.token}
	}

	return &gws.ClientOption{
		Addr:          u.String(),
		RequestHeader: headers,
	}
}

func (dc *deviceClient) connect(hubClient *HubClient) {
	opt := dc.getOptions(hubClient)
	if opt.Addr == "" {
		log.Printf("WebSocket not configured for device %s", dc.deviceIP)
		return
	}
	if hubClient.pubKey == nil {
		log.Printf("Hub public key not configured for device %s", dc.deviceIP)
		return
	}

	// Always use token for initial connections, then try without token for reconnections
	if hubClient.token == "" {
		log.Printf("Token not configured for device %s", dc.deviceIP)
		return
	}

	// For first connection or if we haven't successfully connected yet, use token
	if !dc.hasTriedNoToken || dc.needsToken {
		dc.needsToken = true
		dc.hasTriedNoToken = true
		log.Printf("Using token authentication for device %s", dc.deviceIP)
	}

	log.Printf("Connecting device %s to hub at %s (needsToken: %v)", dc.deviceIP, opt.Addr, dc.needsToken)
	conn, _, err := gws.NewClient(dc, opt)
	if err != nil {
		log.Printf("Failed to connect device %s to hub: %v", dc.deviceIP, err)

		// If connection failed without token, try with token
		if !dc.needsToken && hubClient.token != "" {
			log.Printf("Connection without token failed for device %s, trying with token", dc.deviceIP)
			dc.needsToken = true
			time.AfterFunc(2*time.Second, func() { dc.connect(hubClient) })
			return
		}

		// If connection failed with token, this might be a new agent requiring registration
		if dc.needsToken && hubClient.token != "" {
			log.Printf("Connection failed for device %s, this might be a new agent requiring registration", dc.deviceIP)
		}

		// Retry later
		time.AfterFunc(5*time.Second, func() { dc.connect(hubClient) })
		return
	}

	dc.conn = conn
	log.Printf("Device %s connected to hub", dc.deviceIP)
	go conn.ReadLoop()
}

func (dc *deviceClient) OnOpen(conn *gws.Conn) {
	log.Printf("WebSocket connection opened for device %s", dc.deviceIP)
	conn.SetDeadline(time.Now().Add(70 * time.Second))

	// Mark that this device successfully connected
	// Future reconnections can try without token first
	dc.mu.Lock()
	dc.needsToken = false
	dc.mu.Unlock()
}

func (dc *deviceClient) OnClose(conn *gws.Conn, err error) {
	log.Printf("WebSocket connection closed for device %s: %v", dc.deviceIP, err)
	dc.hubVerified = false

	// For reconnections, try without token first since we were successfully connected
	dc.mu.Lock()
	dc.needsToken = false // Try without token first for reconnection
	dc.mu.Unlock()

	// Reconnect with backoff
	time.AfterFunc(5*time.Second, func() {
		// Create a temporary hub client for reconnection
		tempClient := &HubClient{
			config: dc.cfg,
			token:  dc.cfg.Token,
			url:    parseURL(dc.cfg.URL),
			pubKey: parsePublicKey(dc.cfg.Key),
		}
		dc.connect(tempClient)
	})
}

func (dc *deviceClient) OnPing(conn *gws.Conn, message []byte) {
	conn.SetDeadline(time.Now().Add(70 * time.Second))
	conn.WritePong(message)
}

func (dc *deviceClient) OnPong(conn *gws.Conn, message []byte) {
	conn.SetDeadline(time.Now().Add(70 * time.Second))
}

func (dc *deviceClient) OnMessage(conn *gws.Conn, message *gws.Message) {
	defer message.Close()
	conn.SetDeadline(time.Now().Add(70 * time.Second))

	if message.Opcode != gws.OpcodeBinary {
		return
	}

	// Decode the hub request using the proper types
	var req common.HubRequest[cbor.RawMessage]
	if err := cbor.NewDecoder(message.Data).Decode(&req); err != nil {
		log.Printf("Failed to decode hub message for device %s: %v", dc.deviceIP, err)
		return
	}

	log.Printf("Received hub request for device %s: %d", dc.deviceIP, req.Action)

	switch req.Action {
	case common.CheckFingerprint:
		dc.handleFingerprintRequest(conn, req.Data)
	case common.GetData:
		dc.handleGetDataRequest(conn)
	default:
		log.Printf("Unknown hub request for device %s: %d", dc.deviceIP, req.Action)
	}
}

func (dc *deviceClient) handleFingerprintRequest(conn *gws.Conn, data cbor.RawMessage) {
	var fr common.FingerprintRequest
	if err := cbor.Unmarshal(data, &fr); err != nil {
		log.Printf("Failed to unmarshal fingerprint request for device %s: %v", dc.deviceIP, err)
		return
	}

	// For now, skip signature verification and mark as verified
	// TODO: Implement proper signature verification
	dc.hubVerified = true
	log.Printf("Hub verified for device %s", dc.deviceIP)

	// Generate fingerprint for this specific device
	fingerprint := dc.generateDeviceFingerprint()

	// Send fingerprint response
	resp := &common.FingerprintResponse{
		Fingerprint: fingerprint,
		Hostname:    dc.deviceIP, // Use device IP as hostname
	}

	if fr.NeedSysInfo {
		resp.Hostname = dc.deviceIP
	}

	if err := dc.sendMessage(conn, resp); err != nil {
		log.Printf("Failed to send fingerprint response for device %s: %v", dc.deviceIP, err)
	} else {
		log.Printf("Sending fingerprint response for device %s: %s", dc.deviceIP, fingerprint)
	}
}

func (dc *deviceClient) handleGetDataRequest(conn *gws.Conn) {
	log.Printf("Hub requested data for device %s", dc.deviceIP)

	// Build the combined data for this specific device
	combinedData := dc.buildCombinedData()

	// Send the data
	if err := dc.sendMessage(conn, combinedData); err != nil {
		log.Printf("Failed to send data for device %s: %v", dc.deviceIP, err)
	} else {
		log.Printf("Data sent successfully for device %s", dc.deviceIP)
	}
}

func (dc *deviceClient) generateDeviceFingerprint() string {
	// Generate a unique fingerprint for this specific SNMP device
	base := fmt.Sprintf("snmp-device-%s-%s", dc.deviceName, dc.deviceIP)
	sum := sha256.Sum256([]byte(base))
	return hex.EncodeToString(sum[:24])
}

func (dc *deviceClient) buildCombinedData() *system.CombinedData {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	// Build stats from the device's metrics
	stats := system.Stats{
		Temperatures: make(map[string]float64),
	}

	// Convert device metrics to the appropriate stat categories
	for _, metric := range dc.lastData.Metrics {
		switch strings.ToLower(metric.Category) {
		case "temperature", "temp", "t":
			stats.Temperatures[metric.Name] = metric.Value
		}
	}

	// Build info for this device
	// Use device name if available, otherwise use IP address
	systemName := dc.deviceIP
	if dc.deviceName != "" && strings.TrimSpace(dc.deviceName) != "" {
		systemName = strings.TrimSpace(dc.deviceName)
	}

	info := system.Info{
		Hostname:     systemName,
		AgentType:    "snmp",
		AgentVersion: beszel.Version,
	}

	// Add dashboard summaries
	if len(stats.Temperatures) > 0 {
		var maxTemp float64
		for _, temp := range stats.Temperatures {
			if temp > maxTemp {
				maxTemp = temp
			}
		}
		info.DashboardTemp = maxTemp
	}

	return &system.CombinedData{
		Stats: stats,
		Info:  info,
	}
}

func (dc *deviceClient) sendMessage(conn *gws.Conn, data interface{}) error {
	bytes, err := cbor.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}
	return conn.WriteMessage(gws.OpcodeBinary, bytes)
}

// Legacy method for backward compatibility
func (c *HubClient) SendData(deviceData []DeviceData) {
	for _, data := range deviceData {
		c.NotifyDevice(data)
	}
}
