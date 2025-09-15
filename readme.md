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

## SNMP hub agent

This repository includes an optional SNMP hub agent that listens for/polls SNMP devices and forwards metrics to the Beszel hub as a "SNMP" agent. OS charts are hidden for SNMP agents; only relevant sensor charts are shown.

- Source: `internal/cmd/snmp-hub-agent`
- Config example: `configs/snmp-hub.v2c.example.json`

Build only the SNMP hub agent (no UI build required):

- make build-snmp-hub-agent
  - Output: `./build/beszel-snmp-agent_<os>_<arch>`

Run:

- ./build/beszel-snmp-agent_<os>_<arch> ./configs/snmp-hub.v2c.example.json

Config highlights (v2c):
- `hub.url` – Hub URL (e.g., http://<hub>:8090)
- `hub.token` – Hub API token
- `hub.key` – Public key used to register the system
- `defaults.*` – Global polling and listener settings
- `devices[].oids` – Map of OID -> { name, kind, unit, category, scale }
  - category accepts: temperature, humidity, co2, pressure, pm25, pm10, voc
  - unit should match the list above where applicable

## Building and publishing a custom Hub image

You can build and publish your own Hub image (for example, to Docker Hub under `chaugan/beszel`) and use it in unRaid.

1. Build the web UI (required for embedding):
   - With bun:
     - bun install --cwd ./internal/site
     - bun run --cwd ./internal/site build
   - Or with npm:
     - npm ci --prefix ./internal/site
     - npm run --prefix ./internal/site build

2. Log in to Docker Hub:
   - docker login -u <your_dockerhub_username>

3. Build and push the image:
   - Single-arch (amd64):
     - docker build -f ./internal/dockerfile_hub -t chaugan/beszel:snmp-dev .
     - docker push chaugan/beszel:snmp-dev
   - Multi-arch (recommended):
     - docker buildx create --use
     - docker buildx build --platform linux/amd64,linux/arm64 -f ./internal/dockerfile_hub -t chaugan/beszel:latest -t chaugan/beszel:snmp-YYYYMMDD --push .

4. Use in unRaid:
   - Set the container Repository to `chaugan/beszel:latest` (or your chosen tag) in the template and apply updates.

Notes:
- No GitHub fork is required to publish a custom image. A fork is useful for version control and CI.
- The hub UI now detects `AgentType=snmp` and only renders sensor charts that have data.

## Help and discussion

Please search existing issues and discussions before opening a new one. I try my best to respond, but may not always have time to do so.

#### Bug reports and feature requests

Bug reports and detailed feature requests should be posted on [GitHub issues](https://github.com/henrygd/beszel/issues).

#### Support and general discussion

Support requests and general discussion can be posted on [GitHub discussions](https://github.com/henrygd/beszel/discussions) or the community-run [Matrix room](https://matrix.to/#/#beszel:matrix.org): `#beszel:matrix.org`.

## License

Beszel is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.
