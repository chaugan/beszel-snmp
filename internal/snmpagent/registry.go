package snmpagent

import (
	"net"
	"sync"
)

type registry struct {
	mu      sync.RWMutex
	devices map[string]*deviceState // key: ip string
}

func newRegistry() *registry {
	return &registry{devices: map[string]*deviceState{}}
}

func (r *registry) get(ip net.IP) (*deviceState, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ds, ok := r.devices[ip.String()]
	return ds, ok
}

func (r *registry) set(ip net.IP, ds *deviceState) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.devices[ip.String()] = ds
}
