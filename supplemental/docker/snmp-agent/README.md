# Beszel SNMP Agent Docker Setup

This directory contains the Docker setup for the Beszel SNMP Agent, which collects environmental sensor data from SNMP devices and sends it to the Beszel hub.

## Quick Start

1. **Configure your devices** by editing `devices.json`:
   ```json
   {
     "devices": [
       {
         "match": { "ip_regex": "192\\.168\\.1\\.\\d+" },
         "fingerprint": "snmp-%IP%",
         "hostname_template": "%sysName%",
         "poll": true,
         "poll_interval_sec": 15,
         "communities": ["public"],
         "oids": {
           ".1.3.6.1.4.1.9.9.13.1.3.1.3.0": {
             "name": "temperature",
             "kind": "gauge",
             "unit": "°C",
             "category": "temperature",
             "scale": 1
           }
         }
       }
     ]
   }
   ```

2. **Set environment variables** in `docker-compose.yml`:
   ```yaml
   environment:
     - BESZEL_HUB_URL=http://your-hub-ip:8090
     - BESZEL_HUB_TOKEN=your-hub-token
     - BESZEL_HUB_KEY=your-ssh-public-key
   ```

3. **Start the agent**:
   ```bash
   docker-compose up -d
   ```

## Docker Networking

The SNMP agent uses **host networking mode** to preserve the original source IP addresses of SNMP traps. This is important because:

- **Without host networking**: The agent sees traps as coming from Docker's internal IP (e.g., `172.17.0.1`)
- **With host networking**: The agent sees traps as coming from the actual device IP (e.g., `192.168.86.100`)

This ensures that each SNMP device (switch, router, etc.) appears as a separate system in the Beszel hub with its correct IP address and fingerprint.

### Alternative: Bridge Networking

If you prefer to use bridge networking, you can modify the `docker-compose.yml`:

```yaml
services:
  snmp-agent:
    # ... other config ...
    ports:
      - "9162:9162/udp"
    # Remove: network_mode: host
    networks:
      - beszel-network
```

However, this will cause all SNMP devices to appear with the same IP address in the hub.

## Configuration

### Environment Variables

#### Required
- `BESZEL_HUB_URL`: URL of the Beszel hub (e.g., `http://192.168.1.100:8090`)
- `BESZEL_HUB_TOKEN`: Authentication token from the hub
- `BESZEL_HUB_KEY`: SSH public key for authentication

#### Optional (with defaults)
- `BESZEL_SEND_INTERVAL_SEC`: How often to send data to hub (default: 10)
- `BESZEL_POLL_INTERVAL_SEC`: How often to poll SNMP devices (default: 30)
- `BESZEL_RESOLVE_MIBS`: Whether to resolve MIB names (default: false)
- `BESZEL_MIB_PATHS`: Comma-separated MIB file paths (default: `/usr/share/snmp/mibs`)
- `BESZEL_ROUND1`: Round values to 1 decimal place (default: true)
- `BESZEL_LOG_UNKNOWN`: Log unknown OIDs (default: true)
- `BESZEL_COMMUNITIES`: Comma-separated SNMP communities (default: `public`)
- `BESZEL_LISTEN_ADDR`: Address to listen for SNMP traps (default: `:9162`)
- `BESZEL_DEBUG`: Enable debug logging to see heartbeat messages (default: false)

### Device Configuration

The `devices.json` file contains only the device configurations. Each device entry includes:

- `match.ip_regex`: Regular expression to match device IP addresses
- `fingerprint`: Template for device identification (use `%IP%` placeholder)
- `hostname_template`: Template for device hostname (use `%sysName%` placeholder)
- `poll`: Whether to actively poll this device
- `poll_interval_sec`: How often to poll this device
- `communities`: SNMP communities to use for this device
- `oids`: OID mappings for sensor data

### OID Configuration

Each OID entry defines:
- `name`: Internal name for the sensor
- `kind`: Data type (`gauge`, `counter`, etc.)
- `unit`: Unit of measurement
- `category`: Sensor category (`temperature`, `humidity`, `co2`, `pressure`, `pm25`, `pm10`, `voc`)
- `scale`: Scaling factor to apply to the value

## Building the Image

To build the SNMP agent image:

```bash
docker build -f internal/dockerfile_snmp_agent -t beszel-snmp-agent .
```

## Running Standalone

You can also run the agent directly with Docker:

```bash
docker run -d \
  --name beszel-snmp-agent \
  -p 9162:9162/udp \
  -v /path/to/devices.json:/etc/snmp-agent/devices.json:ro \
  -e BESZEL_HUB_URL=http://your-hub:8090 \
  -e BESZEL_HUB_TOKEN=your-token \
  -e BESZEL_HUB_KEY=your-key \
  beszel-snmp-agent
```

## Troubleshooting

### WebSocket Connection Issues

If you see logs like:
```
INFO ws_connect_attempt addr=ws://192.168.86.211:8090/api/beszel/agent-connect ip=192.168.86.243
INFO ws_connected ip=192.168.86.243
INFO hub_verified ip=192.168.86.243
INFO ws_closed ip=192.168.86.243 err="connection closed, code=1000, reason="
```

This indicates the WebSocket connection is closing due to inactivity. The agent now includes a heartbeat mechanism to prevent this issue.

### Check logs
```bash
docker-compose logs -f snmp-agent
```

### Verify configuration
```bash
docker-compose exec snmp-agent cat /etc/snmp-agent/devices.json
```

### Test SNMP connectivity
```bash
docker-compose exec snmp-agent snmpwalk -v2c -c public 192.168.1.100
```

### Debug WebSocket connection
```bash
# Enable debug logging to see heartbeat messages
# Add to docker-compose.yml: - BESZEL_DEBUG=true
# Then restart: docker-compose up -d

# Check if heartbeat is working
docker-compose logs snmp-agent | grep heartbeat

# Monitor connection status
docker-compose logs -f snmp-agent | grep -E "(ws_|hub_)"

# See all debug messages
docker-compose logs -f snmp-agent
```

## Security Notes

- The agent runs as a non-root user inside the container
- SNMP communities should be changed from default `public`
- Use secure networks for SNMP communication
- Consider using SNMPv3 for production environments
