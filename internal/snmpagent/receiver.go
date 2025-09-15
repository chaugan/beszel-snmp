package snmpagent

import (
	"log/slog"
	"net"
	"strings"

	gosnmp "github.com/gosnmp/gosnmp"
)

type trapReceiver struct {
	cfg *Config
	r   *registry
	p   *poller
	c   *clientFactory
}

func newTrapReceiver(cfg *Config, r *registry, p *poller, c *clientFactory) *trapReceiver {
	return &trapReceiver{cfg: cfg, r: r, p: p, c: c}
}

func (tr *trapReceiver) Run() error {
	l := gosnmp.NewTrapListener()
	l.OnNewTrap = tr.handle
	l.Params = gosnmp.Default
	addr := tr.cfg.Defaults.ListenAddr
	slog.Info("snmp_trap_listen", "addr", addr)
	return l.Listen(addr)
}

func (tr *trapReceiver) handle(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
	ip := addr.IP
	// Basic visibility
	slog.Info("snmp_trap_received", "from", ip.String(), "vars", len(packet.Variables))

	var sysName string
	for _, v := range packet.Variables {
		if v.Name == ".1.3.6.1.2.1.1.5.0" { // sysName.0
			switch t := v.Value.(type) {
			case string:
				sysName = t
			case []byte:
				sysName = string(t)
			}
		}
	}
	ds, ok := tr.r.get(ip)
	if !ok {
		// derive with matching device config templates if available
		var dc *DeviceConfig
		if tr.cfg != nil {
			dc = tr.cfg.MatchDevice(ip)
		}
		fpTmpl := ""
		hostTmpl := ""
		if dc != nil {
			fpTmpl = dc.FingerprintTmpl
			hostTmpl = dc.HostnameTemplate
		}
		finger := deriveFingerprint(ip, sysName, fpTmpl)
		host := sysName
		if hostTmpl != "" {
			host = replacePlaceholders(hostTmpl, ip, sysName)
		} else if host == "" {
			host = ip.String()
		}
		ds = newDeviceState(ip, finger, host)
		tr.r.set(ip, ds)
		tr.p.ensurePolling(ip)
	}

	// Known meta OIDs we don't try to map
	const (
		metaSysUpTime  = ".1.3.6.1.2.1.1.3.0"
		metaTrapOID    = ".1.3.6.1.6.3.1.1.4.1.0"
		metaSysName    = ".1.3.6.1.2.1.1.5.0"
	)

	for _, v := range packet.Variables {
		// skip meta oids
		if v.Name == metaSysUpTime || v.Name == metaTrapOID || v.Name == metaSysName {
			continue
		}
		om, found := findOIDMap(tr.cfg, ip, v.Name)
		if !found {
			logUnknownOID(v.Name, v.Value, tr.cfg.Defaults.LogUnknown)
			continue
		}
		if !shouldSend(om) {
			continue
		}
		val := gosnmp.ToBigInt(v.Value)
		if val == nil {
			continue
		}
		f := transformValue(om.Scale, float64(val.Int64()), tr.cfg.Defaults.Round1)
		ds.mu.Lock()
		ds.setMetric(om.Category, om.Name, f)
		ds.mu.Unlock()
		slog.Info("snmp_value_mapped", "ip", ip.String(), "oid", v.Name, "metric", om.Name, "category", om.Category, "value", f)
	}

	// Schedule immediate send via client factory
	tr.c.Notify(ip)
}

func findOIDMap(cfg *Config, ip net.IP, oid string) (OIDMap, bool) {
	// normalize leading dot
	noDot := strings.TrimPrefix(oid, ".")
	for _, d := range cfg.Devices {
		if d.ipRegex != nil && !d.ipRegex.MatchString(ip.String()) {
			continue
		}
		// try both variants to be tolerant to config styles
		if om, ok := d.Oids[noDot]; ok {
			return om, true
		}
		if om, ok := d.Oids["."+noDot]; ok {
			return om, true
		}
	}
	return OIDMap{}, false
}
