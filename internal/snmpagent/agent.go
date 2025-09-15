package snmpagent

import (
	"log/slog"
	"net"
)

type Agent struct {
	cfg *Config
	r   *registry
	p   *poller
	tr  *trapReceiver
	cf  *clientFactory
}

func NewAgent(cfg *Config) (*Agent, error) {
	r := newRegistry()
	cf := newClientFactory(cfg, r)
	p := newPoller(cfg, r, cf)
	tr := newTrapReceiver(cfg, r, p, cf)
	return &Agent{cfg: cfg, r: r, p: p, tr: tr, cf: cf}, nil
}

func (a *Agent) Run() error {
	// start trap receiver (blocks)
	return a.tr.Run()
}

// For tests and future use
func (a *Agent) EnsureDevice(ip string, hostname string) {
	nip := net.ParseIP(ip)
	if nip == nil {
		return
	}
	ds := newDeviceState(nip, deriveFingerprint(nip, hostname, ""), hostname)
	a.r.set(nip, ds)
	a.p.ensurePolling(nip)
	a.cf.Notify(nip)
	slog.Info("device_added", "ip", ip, "host", hostname)
}
