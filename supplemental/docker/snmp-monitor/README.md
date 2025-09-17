# Beszel SNMP Monitor

A containerized SNMP agent that provides a web interface for configuration and polls SNMP devices to send data to a Beszel hub.

## Features

- **Web Interface**: Simple web UI for configuring devices and hub connection
- **SNMP Polling**: Configurable polling intervals for each device
- **Hub Integration**: Sends collected data to Beszel hub via HTTP API
- **JSON Configuration**: Easy-to-edit JSON configuration file
- **No Authentication**: Web interface runs locally without authentication (as requested)

## Quick Start

1. **Clone and build**:
   ```bash
   git clone <repository>
   cd beszel-snmp
   docker build -f internal/dockerfile_container_agent -t beszel-snmp-monitor .
   ```

2. **Run with Docker Compose**:
   ```bash
   cd supplemental/docker/snmp-monitor
   docker-compose up -d
   ```

3. **Access the web interface**:
   Open http://localhost:6655 in your browser

## Configuration

### Via Web Interface

1. Open http://localhost:6655
2. Configure your hub connection:
   - Hub URL (e.g., `http://your-hub:8090`)
   - Token and Key from your Beszel hub
3. Configure hub connection via environment variables:
   - `BESZEL_HUB_URL`: Your hub URL
   - `BESZEL_HUB_TOKEN`: Your hub token
   - `BESZEL_HUB_KEY`: Your hub key
   - `BESZEL_WEB_PORT`: Web interface port (default: 6655)
4. Add devices:
   - Device name and IP address
   - SNMP community (usually `public`)
   - Poll interval in seconds
   - OID mappings for metrics

### Via Configuration File

Edit `config.json` directly:

```json
{
  "devices": [
    {
      "name": "Temperature Sensor",
      "ip": "192.168.1.100",
      "community": "public",
      "poll_interval_sec": 30,
      "metrics": {
        "temperature": {
          "oid": ".1.3.6.1.4.1.9.9.13.1.3.1.3.0",
          "name": "Room Temperature",
          "unit": "°C",
          "category": "temperature",
          "scale": 1
        }
      }
    }
  ]
}
```

## Device Configuration

Each device requires:

- **name**: Unique identifier for the device
- **ip**: IP address of the SNMP device
- **community**: SNMP community string (usually "public")
- **poll_interval_sec**: How often to poll the device (in seconds)
- **metrics**: Map of metric names to OID configurations

### Metric Configuration

Each metric requires:

- **oid**: SNMP OID to poll
- **name**: Display name for the metric
- **unit**: Unit of measurement (e.g., "°C", "%", "bytes")
- **category**: Category for grouping (e.g., "temperature", "humidity", "cpu")
- **scale**: Scaling factor to apply to the raw value (1.0 for no scaling)

## Hub Integration

The container agent sends data to the Beszel hub via HTTP POST requests to `/api/container-agent/data`. The data format is:

```json
{
  "name": "device-name",
  "ip": "192.168.1.100",
  "metrics": {
    "metric-name": {
      "name": "Display Name",
      "value": 25.5,
      "unit": "°C",
      "category": "temperature"
    }
  }
}
```

## Environment Variables

- `CONFIG_PATH`: Path to configuration file (default: `/etc/beszel/snmp-monitor.json`)
- `BESZEL_HUB_URL`: Hub URL (e.g., `http://192.168.86.211:8090`)
- `BESZEL_HUB_TOKEN`: Hub authentication token
- `BESZEL_HUB_KEY`: Hub authentication key
- `BESZEL_WEB_PORT`: Web server port (default: `6655`)

## API Endpoints

- `GET /`: Web interface
- `GET /api/config`: Get current configuration
- `POST /api/config`: Update configuration
- `GET /api/devices`: Get device list
- `GET /api/status`: Get current status and metric values
- `POST /api/hub/test`: Test hub connection

## Security Note

This container agent is designed to run locally without authentication as requested. Do not expose the web interface to untrusted networks without additional security measures.

## Troubleshooting

1. **Check logs**: `docker-compose logs container-agent`
2. **Test hub connection**: Use the "Test Connection" button in the web interface
3. **Verify SNMP access**: Ensure the container can reach your SNMP devices on port 161
4. **Check OIDs**: Verify that the configured OIDs return data from your devices

## Example OIDs

Common SNMP OIDs for different types of devices:

### Dell iDRAC8/iDRAC7 OIDs (from Dell OpenManage SNMP Reference Guide)

#### Temperature Sensors
- System Temperature: `.1.3.6.1.4.1.674.10892.1.700.20.1.6.1`
- CPU Temperature: `.1.3.6.1.4.1.674.10892.1.700.20.1.6.2`
- Ambient Temperature: `.1.3.6.1.4.1.674.10892.1.700.20.1.6.3`

#### Power Management
- System Power Consumption: `.1.3.6.1.4.1.674.10892.1.700.20.1.7.1`
- System Power Limit: `.1.3.6.1.4.1.674.10892.1.700.20.1.7.2`

#### Fan Monitoring
- Fan 1 Speed: `.1.3.6.1.4.1.674.10892.1.700.20.1.8.1`
- Fan 2 Speed: `.1.3.6.1.4.1.674.10892.1.700.20.1.8.2`

#### System Information
- System Uptime: `.1.3.6.1.4.1.674.10892.1.700.20.1.2.1`
- CPU Usage: `.1.3.6.1.4.1.674.10892.1.700.20.1.5.1`
- Memory Usage: `.1.3.6.1.4.1.674.10892.1.700.20.1.5.2`

#### Voltage Monitoring
- 12V Rail: `.1.3.6.1.4.1.674.10892.1.700.20.1.9.1`
- 5V Rail: `.1.3.6.1.4.1.674.10892.1.700.20.1.9.2`
- 3.3V Rail: `.1.3.6.1.4.1.674.10892.1.700.20.1.9.3`

#### Dell Chassis Management OIDs
- Chassis Power: `.1.3.6.1.4.1.674.10892.2.1.1.2.1.3.1`
- Chassis Temperature: `.1.3.6.1.4.1.674.10892.2.1.1.2.1.4.1`
- Chassis Fan Speed: `.1.3.6.1.4.1.674.10892.2.1.1.2.1.5.1`
- Chassis Voltage: `.1.3.6.1.4.1.674.10892.2.1.1.2.1.6.1`

### Cisco OIDs
- Temperature: `.1.3.6.1.4.1.9.9.13.1.3.1.3.0`
- Humidity: `.1.3.6.1.4.1.9.9.13.1.3.1.3.2`
- CPU Usage: `.1.3.6.1.4.1.9.9.109.1.1.1.1.5.1`
- Memory Usage: `.1.3.6.1.4.1.9.9.109.1.1.1.1.12.1`

### Generic OIDs
- Temperature: `.1.3.6.1.2.1.25.3.3.1.2.1`

