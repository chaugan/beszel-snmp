# Beszel

Beszel is a lightweight server monitoring platform that includes Docker statistics, historical data, and alert functions.

It has a friendly web interface, simple configuration, and is ready to use out of the box. It supports automatic backup, multi-user, OAuth authentication, and API access.

[![agent Docker Image Size](https://img.shields.io/docker/image-size/henrygd/beszel-agent/latest?logo=docker&label=agent%20image%20size)](https://hub.docker.com/r/henrygd/beszel-agent)
[![hub Docker Image Size](https://img.shields.io/docker/image-size/henrygd/beszel/latest?logo=docker&label=hub%20image%20size)](https://hub.docker.com/r/henrygd/beszel)
[![MIT license](https://img.shields.io/github/license/henrygd/beszel?color=%239944ee)](https://github.com/henrygd/beszel/blob/main/LICENSE)
[![Crowdin](https://badges.crowdin.net/beszel/localized.svg)](https://crowdin.com/project/beszel)

![Screenshot of Beszel dashboard and system page, side by side. The dashboard shows metrics from multiple connected systems, while the system page shows detailed metrics for a single system.](https://henrygd-assets.b-cdn.net/beszel/screenshot-new.png)

## Features

- **Lightweight**: Smaller and less resource-intensive than leading solutions.
- **Simple**: Easy setup with little manual configuration required.
- **Docker stats**: Tracks CPU, memory, and network usage history for each container.
- **SNMP Support**: Monitor environmental sensors (temperature, humidity, CO2, pressure, PM2.5, PM10, VOC) via SNMP.
- **Alerts**: Configurable alerts for CPU, memory, disk, bandwidth, temperature, load average, status, and SNMP sensors.
- **Multi-user**: Users manage their own systems. Admins can share systems across users.
- **OAuth / OIDC**: Supports many OAuth2 providers. Password auth can be disabled.
- **Automatic backups**: Save to and restore from disk or S3-compatible storage.
<!-- - **REST API**: Use or update your data in your own scripts and applications. -->

## Architecture

Beszel consists of two main components: the **hub** and the **agent**.

- **Hub**: A web application built on [PocketBase](https://pocketbase.io/) that provides a dashboard for viewing and managing connected systems.
- **Agent**: Runs on each system you want to monitor and communicates system metrics to the hub.

## Getting started

The [quick start guide](https://beszel.dev/guide/getting-started) and other documentation is available on our website, [beszel.dev](https://beszel.dev). You'll be up and running in a few minutes.

## Screenshots

![Dashboard](https://beszel.dev/image/dashboard.png)
![System page](https://beszel.dev/image/system-full.png)
![Notification Settings](https://beszel.dev/image/settings-notifications.png)

## Supported metrics

- **CPU usage** - Host system and Docker / Podman containers.
- **Memory usage** - Host system and containers. Includes swap and ZFS ARC.
- **Disk usage** - Host system. Supports multiple partitions and devices.
- **Disk I/O** - Host system. Supports multiple partitions and devices.
- **Network usage** - Host system and containers.
- **Load average** - Host system.
- **Temperature** - Host system sensors.
- **GPU usage / temperature / power draw** - Nvidia and AMD only. Must use binary agent.
- **Battery** - Host system battery charge.

### Additional sensors via SNMP agent (extended)

When using the included SNMP hub agent, Beszel can ingest additional environmental sensor categories. The hub UI only shows charts for categories that have data.

Supported categories and units:
- Temperature — °C / °F
- Humidity — %
- CO2 — ppm
- Pressure — hPa
- PM2.5 — µg/m³
- PM10 — µg/m³
- VOC — ppb

UI behavior in All Systems:
- Main table lists standard agents (CPU/memory/etc.).
- SNMP systems are listed in a separate "SNMP Sensors" table below, with columns for the categories above.
- Cells populate only when the system has data in that category (blank otherwise).

## SNMP Monitor

This repository includes an SNMP Monitor that can be deployed as a Docker container to poll SNMP devices and forward metrics to the Beszel hub. The monitor includes a web interface for easy configuration management.

### Features

- **Web Interface**: Configure SNMP devices, hub settings, and polling intervals through a web UI
- **Per-Device Connections**: Each monitored device gets its own connection to the hub with unique fingerprints
- **Dynamic Configuration**: Changes made through the web interface take effect immediately
- **Environment Variable Support**: Can be configured via environment variables or web interface
- **Docker Ready**: Pre-built Docker image available on Docker Hub

### Quick Start

#### Using Docker

1. **Pull the image**:
   ```bash
   docker pull chaugan/beszel-snmp-monitor:latest
   ```

2. **Run with environment variables**:
   ```bash
   docker run -d \
     --name beszel-snmp-monitor \
     -p 6655:6655 \
     -e BESZEL_HUB_URL=http://your-hub:8090 \
     -e BESZEL_HUB_TOKEN=your-token \
     -e BESZEL_HUB_KEY=your-ssh-key \
     -v ./config.json:/etc/beszel/snmp-monitor.json \
     chaugan/beszel-snmp-monitor:latest
   ```

3. **Access the web interface**:
   - Open `http://your-server:6655` in your browser
   - Configure your SNMP devices and hub settings
   - Changes take effect immediately when saved

#### Configuration

The SNMP Monitor supports configuration through:

- **Web Interface** (recommended): Access the web UI to configure devices and settings
- **Environment Variables**: Set `BESZEL_HUB_URL`, `BESZEL_HUB_TOKEN`, `BESZEL_HUB_KEY`, `BESZEL_WEB_PORT`
- **JSON Config File**: Mount a custom config file to `/etc/beszel/snmp-monitor.json`

#### Configuration Format

```json
{
  "hub": {
    "url": "http://your-hub:8090",
    "token": "your-hub-token",
    "key": "your-ssh-public-key"
  },
  "web_server": {
    "port": 6655
  },
  "devices": [
    {
      "name": "Dell iDRAC Server",
      "ip": "192.168.1.100",
      "community": "public",
      "poll_interval_sec": 30,
      "metrics": {
        "cpu_temperature": {
          "oid": ".1.3.6.1.4.1.674.10892.1.700.20.1.6.2",
          "name": "CPU Temperature",
          "unit": "°C",
          "category": "temperature",
          "scale": 0.1
        }
      }
    }
  ]
}
```

#### Supported Categories

- **temperature** - Temperature sensors (°C/°F)
- **humidity** - Humidity sensors (%)
- **co2** - CO2 sensors (ppm)
- **pressure** - Pressure sensors (hPa)
- **pm25** - PM2.5 sensors (µg/m³)
- **pm10** - PM10 sensors (µg/m³)
- **voc** - VOC sensors (ppb)

The hub UI detects `AgentType=snmp` and only renders sensor charts that have data. Each device appears as a separate system in the Beszel UI with its own IP address and unique fingerprint.

## Help and discussion

Please search existing issues and discussions before opening a new one. I try my best to respond, but may not always have time to do so.

#### Bug reports and feature requests

Bug reports and detailed feature requests should be posted on [GitHub issues](https://github.com/henrygd/beszel/issues).

#### Support and general discussion

Support requests and general discussion can be posted on [GitHub discussions](https://github.com/henrygd/beszel/discussions) or the community-run [Matrix room](https://matrix.to/#/#beszel:matrix.org): `#beszel:matrix.org`.

## License

Beszel is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.
