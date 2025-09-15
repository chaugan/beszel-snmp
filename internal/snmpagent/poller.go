package snmpagent

import (
	"log/slog"
	"net"
	"time"

	gosnmp "github.com/gosnmp/gosnmp"
)

type poller struct {
	cfg *Config
	r   *registry
	c   *clientFactory
}

func newPoller(cfg *Config, r *registry, c *clientFactory) *poller {
	return &poller{cfg: cfg, r: r, c: c}
}

func (p *poller) ensurePolling(ip net.IP) {
	// Find matching device rule with poll enabled
	var dc *DeviceConfig
	for i := range p.cfg.Devices {
		d := &p.cfg.Devices[i]
		if d.Poll && (d.ipRegex == nil || d.ipRegex.MatchString(ip.String())) {
			dc = d
			break
		}
	}
	if dc == nil {
		return
	}
	interval := time.Duration(dc.PollIntervalSec) * time.Second
	go p.pollLoop(ip, interval, dc)
}

func (p *poller) pollLoop(ip net.IP, interval time.Duration, dc *DeviceConfig) {
	for {
		p.pollOnce(ip, dc)
		time.Sleep(interval)
	}
}

func (p *poller) pollOnce(ip net.IP, dc *DeviceConfig) {
	params := &gosnmp.GoSNMP{
		Target:    ip.String(),
		Port:      161,
		Community: firstCommunity(dc, p.cfg),
		Version:   gosnmp.Version2c,
		Timeout:   2 * time.Second,
		Retries:   1,
	}
	if err := params.Connect(); err != nil {
		slog.Debug("snmp_connect_fail", "ip", ip, "err", err)
		return
	}
	defer params.Conn.Close()

	for oid, om := range dc.Oids {
		if !shouldSend(om) {
			continue
		}
		res, err := params.Get([]string{oid})
		if err != nil || len(res.Variables) == 0 {
			continue
		}
		v := res.Variables[0]
		bi := gosnmp.ToBigInt(v.Value)
		if bi == nil {
			continue
		}
		f := transformValue(om.Scale, float64(bi.Int64()), p.cfg.Defaults.Round1)
		if ds, ok := p.r.get(ip); ok {
			ds.mu.Lock()
			ds.setMetric(om.Category, om.Name, f)
			ds.mu.Unlock()
		}
	}
	p.c.Notify(ip)
}

func firstCommunity(dc *DeviceConfig, cfg *Config) string {
	if len(dc.Communities) > 0 {
		return dc.Communities[0]
	}
	if len(cfg.Defaults.Communities) > 0 {
		return cfg.Defaults.Communities[0]
	}
	return "public"
}
