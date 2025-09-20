package snmpmonitor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

// WebServer handles the web interface for configuration
type WebServer struct {
	agent  *Agent
	config *WebServerConfig
	mux    *http.ServeMux
}

// NewWebServer creates a new web server
func NewWebServer(agent *Agent, config *WebServerConfig) (*WebServer, error) {
	ws := &WebServer{
		agent:  agent,
		config: config,
		mux:    http.NewServeMux(),
	}

	ws.setupRoutes()
	return ws, nil
}

// setupRoutes sets up the HTTP routes
func (ws *WebServer) setupRoutes() {
	// Serve static files (CSS, JS, etc.)
	ws.mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

	// API routes
	ws.mux.HandleFunc("/api/config", ws.handleConfig)
	ws.mux.HandleFunc("/api/devices", ws.handleDevices)
	ws.mux.HandleFunc("/api/status", ws.handleStatus)
	ws.mux.HandleFunc("/api/hub/test", ws.handleHubTest)

	// Web interface
	ws.mux.HandleFunc("/", ws.handleIndex)
}

// Start starts the web server
func (ws *WebServer) Start() error {
	port := ws.config.Port
	addr := fmt.Sprintf(":%d", port)
	log.Printf("Web server listening on %s", addr)
	return http.ListenAndServe(addr, ws.mux)
}

// handleIndex serves the main web interface
func (ws *WebServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Beszel Container Agent</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .header { border-bottom: 2px solid #e0e0e0; padding-bottom: 20px; margin-bottom: 30px; }
        .section { margin-bottom: 30px; }
        .section h2 { color: #333; border-bottom: 1px solid #ddd; padding-bottom: 10px; }
        .form-group { margin-bottom: 15px; }
        .form-group label { display: block; margin-bottom: 5px; font-weight: bold; }
        .form-group input, .form-group select, .form-group textarea { width: 100%; padding: 8px; border: 1px solid #ddd; border-radius: 4px; }
        .form-group textarea { height: 200px; font-family: monospace; }
        .btn { background: #007bff; color: white; padding: 10px 20px; border: none; border-radius: 4px; cursor: pointer; margin-right: 10px; }
        .btn:hover { background: #0056b3; }
        .btn-success { background: #28a745; }
        .btn-success:hover { background: #1e7e34; }
        .btn-danger { background: #dc3545; }
        .btn-danger:hover { background: #c82333; }
        .status { 
            padding: 12px 16px; 
            border-radius: 6px; 
            margin-bottom: 20px; 
            font-weight: 500;
            position: relative;
            animation: slideDown 0.3s ease-out;
        }
        .status.success { 
            background: #d4edda; 
            color: #155724; 
            border: 1px solid #c3e6cb; 
            border-left: 4px solid #28a745;
        }
        .status.error { 
            background: #f8d7da; 
            color: #721c24; 
            border: 1px solid #f5c6cb; 
            border-left: 4px solid #dc3545;
        }
        @keyframes slideDown {
            from { opacity: 0; transform: translateY(-10px); }
            to { opacity: 1; transform: translateY(0); }
        }
        @keyframes slideUp {
            from { opacity: 1; transform: translateY(0); }
            to { opacity: 0; transform: translateY(-10px); }
        }
        .device-list { border: 1px solid #ddd; border-radius: 4px; }
        .device-item { padding: 15px; border-bottom: 1px solid #eee; }
        .device-item:last-child { border-bottom: none; }
        .device-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px; }
        .device-name { font-weight: bold; color: #333; }
        .device-ip { color: #666; }
        .metric-grid { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 10px; margin-top: 10px; }
        .metric { background: #f8f9fa; padding: 10px; border-radius: 4px; border-left: 3px solid #007bff; }
        .metric-name { font-weight: bold; }
        .metric-value { color: #007bff; font-size: 1.1em; }
        .hidden { display: none; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Beszel Container Agent</h1>
            <p>Configure and monitor SNMP devices</p>
        </div>

        <div id="status"></div>

        <div class="section">
            <h2>Hub Configuration</h2>
            <form id="hubConfig">
                <div class="form-group">
                    <label for="hubUrl">Hub URL:</label>
                    <input type="url" id="hubUrl" name="url" placeholder="http://hub.example.com">
                </div>
                <div class="form-group">
                    <label for="hubToken">Token:</label>
                    <input type="text" id="hubToken" name="token" placeholder="Your hub token">
                </div>
                <div class="form-group">
                    <label for="hubKey">Key:</label>
                    <input type="text" id="hubKey" name="key" placeholder="Your hub key">
                </div>
                <button type="submit" class="btn">Save Hub Config</button>
                <button type="button" class="btn btn-success" onclick="testHubConnection()">Test Connection</button>
            </form>
        </div>

        <div class="section">
            <h2>Devices</h2>
            <div id="devices"></div>
            <button class="btn" onclick="addDevice()">Add Device</button>
            <button class="btn btn-success" onclick="saveAllDevices()">Save All Devices</button>
        </div>

        <div class="section">
            <h2>Current Status</h2>
            <div id="statusInfo"></div>
        </div>

        <div class="section">
            <h2>Raw Configuration</h2>
            <textarea id="rawConfig" placeholder="Configuration will appear here..."></textarea>
            <br><br>
            <button class="btn" onclick="loadRawConfig()">Load from Text</button>
            <button class="btn btn-success" onclick="saveRawConfig()">Save Configuration</button>
            <button class="btn" onclick="reloadFromServer()">Reload from Server</button>
        </div>
    </div>

    <script>
        let devices = [];
        
        // Load configuration on page load
        document.addEventListener('DOMContentLoaded', function() {
            loadConfig();
            loadStatus();
            setInterval(loadStatus, 5000); // Refresh status every 5 seconds
        });

        async function loadConfig() {
            try {
                const response = await fetch('/api/config');
                const config = await response.json();
                
                // Populate hub config
                document.getElementById('hubUrl').value = config.hub.url || '';
                document.getElementById('hubToken').value = config.hub.token || '';
                document.getElementById('hubKey').value = config.hub.key || '';
                
                devices = config.devices || [];
                renderDevices();
                renderRawConfig();
            } catch (error) {
                showStatus('Error loading configuration: ' + error.message, 'error');
            }
        }

        async function loadStatus() {
            try {
                const response = await fetch('/api/status');
                const status = await response.json();
                renderStatus(status);
            } catch (error) {
                console.error('Error loading status:', error);
            }
        }

        function renderStatus(status) {
            const statusDiv = document.getElementById('statusInfo');
            let html = '<div class="device-list">';
            
            for (const device of status.devices) {
                html += '<div class="device-item">';
                html += '<div class="device-header">';
                html += '<div>';
                const displayName = device.name && device.name.trim() !== '' ? device.name : device.ip;
                html += '<span class="device-name">' + displayName + '</span>';
                if (device.name && device.name.trim() !== '') {
                    html += '<span class="device-ip">(' + device.ip + ')</span>';
                }
                html += '</div>';
                html += '<div>Status: <strong>' + device.status + '</strong></div>';
                html += '</div>';
                
                if (device.metrics) {
                    html += '<div class="metric-grid">';
                    for (const [name, value] of Object.entries(device.metrics)) {
                        html += '<div class="metric">';
                        html += '<div class="metric-name">' + name + '</div>';
                        html += '<div class="metric-value">' + value + '</div>';
                        html += '</div>';
                    }
                    html += '</div>';
                }
                html += '</div>';
            }
            html += '</div>';
            statusDiv.innerHTML = html;
        }

        function renderDevices() {
            const devicesDiv = document.getElementById('devices');
            let html = '<div class="device-list">';
            
            for (let i = 0; i < devices.length; i++) {
                const device = devices[i];
                html += '<div class="device-item">';
                html += '<div class="device-header">';
                html += '<span class="device-name">' + device.name + '</span>';
                html += '<div>';
                html += '<button class="btn btn-success" onclick="saveDevice(' + i + ')" style="margin-right: 10px;">Save Device</button>';
                html += '<button class="btn btn-danger" onclick="removeDevice(' + i + ')">Remove</button>';
                html += '</div>';
                html += '</div>';
                html += '<div class="form-group">';
                html += '<label>Device Name (optional):</label>';
                html += '<input type="text" id="device-name-' + i + '" value="' + device.name + '" placeholder="Leave empty to use IP address">';
                html += '</div>';
                html += '<div class="form-group">';
                html += '<label>IP Address:</label>';
                html += '<input type="text" id="device-ip-' + i + '" value="' + device.ip + '">';
                html += '</div>';
                html += '<div class="form-group">';
                html += '<label>Community:</label>';
                html += '<input type="text" id="device-community-' + i + '" value="' + device.community + '">';
                html += '</div>';
                html += '<div class="form-group">';
                html += '<label>Poll Interval (seconds):</label>';
                html += '<input type="number" id="device-poll-' + i + '" value="' + device.poll_interval_sec + '">';
                html += '</div>';
                html += '<div class="form-group">';
                html += '<label>Metrics (JSON):</label>';
                html += '<textarea id="device-metrics-' + i + '" style="height: 150px;">' + JSON.stringify(device.metrics, null, 2) + '</textarea>';
                html += '</div>';
                html += '</div>';
            }
            html += '</div>';
            devicesDiv.innerHTML = html;
        }

        function renderRawConfig() {
            const config = {
                hub: {
                    url: document.getElementById('hubUrl').value,
                    token: document.getElementById('hubToken').value,
                    key: document.getElementById('hubKey').value
                },
                web_server: {
                    port: 8080
                },
                devices: devices
            };
            document.getElementById('rawConfig').value = JSON.stringify(config, null, 2);
        }

        function addDevice() {
            const newDevice = {
                name: '', // Start with empty name (optional field)
                ip: '',
                community: 'public',
                poll_interval_sec: 30,
                metrics: {}
            };
            devices.push(newDevice);
            renderDevices();
            renderRawConfig();
        }

        function removeDevice(index) {
            devices.splice(index, 1);
            renderDevices();
            renderRawConfig();
        }

        async function saveDevice(index) {
            try {
                // Get values from form fields
                const name = document.getElementById('device-name-' + index).value;
                const ip = document.getElementById('device-ip-' + index).value;
                const community = document.getElementById('device-community-' + index).value;
                const pollInterval = parseInt(document.getElementById('device-poll-' + index).value);
                const metricsText = document.getElementById('device-metrics-' + index).value;
                
                // Validate required fields (device name is optional)
                if (!ip.trim()) {
                    showStatus('IP address is required', 'error');
                    return;
                }
                if (!community.trim()) {
                    showStatus('Community string is required', 'error');
                    return;
                }
                if (isNaN(pollInterval) || pollInterval <= 0) {
                    showStatus('Poll interval must be a positive number', 'error');
                    return;
                }
                
                // Parse and validate metrics JSON
                let metrics;
                try {
                    metrics = JSON.parse(metricsText);
                } catch (parseError) {
                    showStatus('Invalid JSON in metrics: ' + parseError.message, 'error');
                    return;
                }
                
                // Update device in memory
                devices[index] = {
                    name: name.trim(),
                    ip: ip.trim(),
                    community: community.trim(),
                    poll_interval_sec: pollInterval,
                    metrics: metrics
                };
                
                // Save to server
                const response = await fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ devices: devices })
                });
                
                let responseData;
                try {
                    responseData = await response.json();
                } catch (jsonError) {
                    const errorText = await response.text();
                    showStatus('Server error: ' + errorText, 'error');
                    return;
                }
                
                if (response.ok) {
                    showStatus('Device saved successfully', 'success');
                    renderRawConfig(); // Update raw config display
                } else {
                    if (responseData.error) {
                        showStatus(responseData.message + ': ' + responseData.error, 'error');
                    } else {
                        showStatus('Failed to save device', 'error');
                    }
                }
            } catch (error) {
                showStatus('Error saving device: ' + error.message, 'error');
            }
        }

        async function saveAllDevices() {
            try {
                // Collect all device data from form fields
                const updatedDevices = [];
                let hasErrors = false;
                
                for (let i = 0; i < devices.length; i++) {
                    const name = document.getElementById('device-name-' + i).value;
                    const ip = document.getElementById('device-ip-' + i).value;
                    const community = document.getElementById('device-community-' + i).value;
                    const pollInterval = parseInt(document.getElementById('device-poll-' + i).value);
                    const metricsText = document.getElementById('device-metrics-' + i).value;
                    
                    // Validate required fields (device name is optional)
                    if (!ip.trim()) {
                        showStatus('Device ' + (i + 1) + ': IP address is required', 'error');
                        hasErrors = true;
                        continue;
                    }
                    if (!community.trim()) {
                        showStatus('Device ' + (i + 1) + ': community string is required', 'error');
                        hasErrors = true;
                        continue;
                    }
                    if (isNaN(pollInterval) || pollInterval <= 0) {
                        showStatus('Device ' + (i + 1) + ': poll interval must be a positive number', 'error');
                        hasErrors = true;
                        continue;
                    }
                    
                    // Parse and validate metrics JSON
                    let metrics;
                    try {
                        metrics = JSON.parse(metricsText);
                    } catch (parseError) {
                        showStatus('Device ' + (i + 1) + ': invalid JSON in metrics - ' + parseError.message, 'error');
                        hasErrors = true;
                        continue;
                    }
                    
                    updatedDevices.push({
                        name: name.trim(),
                        ip: ip.trim(),
                        community: community.trim(),
                        poll_interval_sec: pollInterval,
                        metrics: metrics
                    });
                }
                
                if (hasErrors) {
                    return; // Don't save if there are validation errors
                }
                
                // Update devices in memory
                devices = updatedDevices;
                
                // Save to server
                const response = await fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ devices: devices })
                });
                
                let responseData;
                try {
                    responseData = await response.json();
                } catch (jsonError) {
                    const errorText = await response.text();
                    showStatus('Server error: ' + errorText, 'error');
                    return;
                }
                
                if (response.ok) {
                    showStatus('All devices saved successfully', 'success');
                    renderRawConfig(); // Update raw config display
                } else {
                    if (responseData.error) {
                        showStatus(responseData.message + ': ' + responseData.error, 'error');
                    } else {
                        showStatus('Failed to save devices', 'error');
                    }
                }
            } catch (error) {
                showStatus('Error saving devices: ' + error.message, 'error');
            }
        }

        // Hub config form submission
        document.getElementById('hubConfig').addEventListener('submit', async function(e) {
            e.preventDefault();
            const formData = new FormData(e.target);
            const hubConfig = {
                url: formData.get('url'),
                token: formData.get('token'),
                key: formData.get('key')
            };
            
            try {
                const response = await fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ hub: hubConfig })
                });
                
                let responseData;
                try {
                    responseData = await response.json();
                } catch (jsonError) {
                    // If response is not JSON, show the raw text
                    const errorText = await response.text();
                    showStatus('Server error: ' + errorText, 'error');
                    return;
                }
                
                if (response.ok) {
                    showStatus(responseData.message || 'Hub configuration saved successfully', 'success');
                } else {
                    // Handle structured error response
                    if (responseData.error) {
                        showStatus(responseData.message + ': ' + responseData.error, 'error');
                    } else {
                        showStatus('Failed to save hub configuration', 'error');
                    }
                }
            } catch (error) {
                showStatus('Error saving hub configuration: ' + error.message, 'error');
            }
        });

        async function testHubConnection() {
            try {
                const response = await fetch('/api/hub/test', { method: 'POST' });
                if (response.ok) {
                    showStatus('Hub connection test successful', 'success');
                } else {
                    throw new Error('Connection test failed');
                }
            } catch (error) {
                showStatus('Hub connection test failed: ' + error.message, 'error');
            }
        }

        async function saveRawConfig() {
            try {
                const configText = document.getElementById('rawConfig').value;
                
                // Validate JSON syntax first
                let config;
                try {
                    config = JSON.parse(configText);
                } catch (parseError) {
                    showStatus('JSON Syntax Error: ' + parseError.message, 'error');
                    return;
                }
                
                const response = await fetch('/api/config', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(config)
                });
                
                let responseData;
                try {
                    responseData = await response.json();
                } catch (jsonError) {
                    // If response is not JSON, show the raw text
                    const errorText = await response.text();
                    showStatus('Server error: ' + errorText, 'error');
                    return;
                }
                
                if (response.ok) {
                    showStatus(responseData.message || 'Configuration saved successfully', 'success');
                    // Don't reload config automatically - let user see their changes
                } else {
                    // Handle structured error response
                    if (responseData.error) {
                        showStatus(responseData.message + ': ' + responseData.error, 'error');
                    } else {
                        showStatus('Failed to save configuration', 'error');
                    }
                }
            } catch (error) {
                showStatus('Error saving configuration: ' + error.message, 'error');
            }
        }

        async function loadRawConfig() {
            try {
                const configText = document.getElementById('rawConfig').value;
                const config = JSON.parse(configText);
                
                // Update form fields
                document.getElementById('hubUrl').value = config.hub?.url || '';
                document.getElementById('hubToken').value = config.hub?.token || '';
                document.getElementById('hubKey').value = config.hub?.key || '';
                
                devices = config.devices || [];
                renderDevices();
                showStatus('Configuration loaded from raw config', 'success');
            } catch (error) {
                showStatus('Error parsing configuration: ' + error.message, 'error');
            }
        }

        async function reloadFromServer() {
            try {
                await loadConfig();
                showStatus('Configuration reloaded from server', 'success');
            } catch (error) {
                showStatus('Error reloading configuration: ' + error.message, 'error');
            }
        }

        function showStatus(message, type) {
            const statusDiv = document.getElementById('status');
            const notificationId = 'notification-' + Date.now();
            statusDiv.innerHTML = '<div id="' + notificationId + '" class="status ' + type + '">' + 
                '<span>' + message + '</span>' +
                '<button onclick="closeNotification(\'' + notificationId + '\')" style="float: right; background: none; border: none; font-size: 18px; cursor: pointer; color: inherit; opacity: 0.7;">&times;</button>' +
                '</div>';
            
            // Auto-hide after 7 seconds for errors, 3 seconds for success
            const hideDelay = type === 'error' ? 7000 : 3000;
            setTimeout(() => {
                const notification = document.getElementById(notificationId);
                if (notification) {
                    notification.style.animation = 'slideUp 0.3s ease-out';
                    setTimeout(() => {
                        notification.remove();
                    }, 300);
                }
            }, hideDelay);
        }

        function closeNotification(notificationId) {
            const notification = document.getElementById(notificationId);
            if (notification) {
                notification.style.animation = 'slideUp 0.3s ease-out';
                setTimeout(() => {
                    notification.remove();
                }, 300);
            }
        }
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

// handleConfig handles configuration API requests
func (ws *WebServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		ws.getConfig(w, r)
	case "POST":
		ws.updateConfig(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getConfig returns the current configuration
func (ws *WebServer) getConfig(w http.ResponseWriter, r *http.Request) {
	// Combine config from JSON file and environment variables
	config := ws.agent.GetConfig()
	hubConfig := ws.agent.GetHubConfig()
	webServerConfig := ws.agent.GetWebServerConfig()

	combinedConfig := struct {
		Hub       *HubConfig       `json:"hub"`
		WebServer *WebServerConfig `json:"web_server"`
		Devices   []DeviceConfig   `json:"devices"`
	}{
		Hub:       hubConfig,
		WebServer: webServerConfig,
		Devices:   config.Devices,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(combinedConfig)
}

// updateConfig updates the configuration
func (ws *WebServer) updateConfig(w http.ResponseWriter, r *http.Request) {
	// Read the raw body first for JSON validation
	body, err := ws.readRequestBody(r)
	if err != nil {
		ws.sendJSONError(w, "Failed to read request body", err, http.StatusBadRequest)
		return
	}

	// Validate JSON syntax
	if !json.Valid(body) {
		ws.sendJSONError(w, "Invalid JSON syntax", fmt.Errorf("malformed JSON"), http.StatusBadRequest)
		return
	}

	// Parse the JSON
	var updateData struct {
		Hub       *HubConfig       `json:"hub"`
		WebServer *WebServerConfig `json:"web_server"`
		Devices   []DeviceConfig   `json:"devices"`
	}

	if err := json.Unmarshal(body, &updateData); err != nil {
		ws.sendJSONError(w, "Failed to parse configuration", err, http.StatusBadRequest)
		return
	}

	// Validate configuration structure
	if err := ws.validateConfiguration(&updateData); err != nil {
		ws.sendJSONError(w, "Configuration validation failed", err, http.StatusBadRequest)
		return
	}

	// Create new config with devices
	newConfig := &Config{
		Devices: updateData.Devices,
	}

	// Add hub and web server config if provided
	if updateData.Hub != nil {
		newConfig.Hub = updateData.Hub
	}
	if updateData.WebServer != nil {
		newConfig.WebServer = updateData.WebServer
	}

	if err := ws.agent.UpdateConfig(newConfig); err != nil {
		ws.sendJSONError(w, "Failed to update config", err, http.StatusInternalServerError)
		return
	}

	// Send success response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success", "message": "Configuration updated successfully"})
}

// handleDevices handles device API requests
func (ws *WebServer) handleDevices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ws.agent.GetConfig().Devices)
}

// handleStatus returns the current status
func (ws *WebServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	config := ws.agent.GetConfig()

	status := struct {
		Devices []DeviceStatus `json:"devices"`
	}{
		Devices: make([]DeviceStatus, len(config.Devices)),
	}

	for i, device := range config.Devices {
		// Get actual status and metrics from poller
		deviceStatus, metrics := ws.agent.GetPollerStatus(device.Name)

		status.Devices[i] = DeviceStatus{
			Name:    device.Name,
			IP:      device.IP,
			Status:  deviceStatus,
			Metrics: metrics,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleHubTest tests the hub connection
func (ws *WebServer) handleHubTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	hubConfig := ws.agent.GetHubConfig()
	_, err := NewHubClient(*hubConfig)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to create hub client: %v", err), http.StatusInternalServerError)
		return
	}

	// Connection test passed (client creation succeeded)
	w.WriteHeader(http.StatusOK)
}

// DeviceStatus represents the status of a device
type DeviceStatus struct {
	Name    string             `json:"name"`
	IP      string             `json:"ip"`
	Status  string             `json:"status"`
	Metrics map[string]float64 `json:"metrics"`
}

// readRequestBody reads and returns the request body
func (ws *WebServer) readRequestBody(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	// Reset the body so it can be read again by json.NewDecoder
	r.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

// sendJSONError sends a JSON error response
func (ws *WebServer) sendJSONError(w http.ResponseWriter, message string, err error, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	errorResponse := map[string]string{
		"status":  "error",
		"message": message,
		"error":   err.Error(),
	}
	json.NewEncoder(w).Encode(errorResponse)
}

// validateConfiguration validates the configuration structure
func (ws *WebServer) validateConfiguration(config *struct {
	Hub       *HubConfig       `json:"hub"`
	WebServer *WebServerConfig `json:"web_server"`
	Devices   []DeviceConfig   `json:"devices"`
}) error {
	// Validate devices
	for i, device := range config.Devices {
		if device.Name == "" {
			return fmt.Errorf("device %d: name is required", i)
		}
		if device.IP == "" {
			return fmt.Errorf("device %d: IP address is required", i)
		}
		if device.Community == "" {
			return fmt.Errorf("device %d: community string is required", i)
		}
		if device.PollInterval <= 0 {
			return fmt.Errorf("device %d: poll interval must be greater than 0", i)
		}

		// Validate metrics
		for metricName, metric := range device.Metrics {
			if metric.OID == "" {
				return fmt.Errorf("device %d, metric '%s': OID is required", i, metricName)
			}
			if metric.Name == "" {
				return fmt.Errorf("device %d, metric '%s': name is required", i, metricName)
			}
			if metric.Category == "" {
				return fmt.Errorf("device %d, metric '%s': category is required", i, metricName)
			}
		}
	}

	// Validate hub config if provided
	if config.Hub != nil {
		if config.Hub.URL == "" {
			return fmt.Errorf("hub URL is required")
		}
		if config.Hub.Token == "" {
			return fmt.Errorf("hub token is required")
		}
		if config.Hub.Key == "" {
			return fmt.Errorf("hub key is required")
		}
	}

	// Validate web server config if provided
	if config.WebServer != nil {
		if config.WebServer.Port <= 0 || config.WebServer.Port > 65535 {
			return fmt.Errorf("web server port must be between 1 and 65535")
		}
	}

	return nil
}
