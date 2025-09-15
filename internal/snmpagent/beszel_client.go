package snmpagent

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/henrygd/beszel"
	"github.com/henrygd/beszel/internal/common"
	"github.com/lxzan/gws"
	gossh "golang.org/x/crypto/ssh"
)

type clientFactory struct {
	cfg    *Config
	r      *registry
	mu     sync.Mutex
	conns  map[string]*deviceClient
	pubKey gossh.PublicKey
	url    *url.URL
	token  string
}

type deviceClient struct {
	gws.BuiltinEventHandler
	ip          net.IP
	cfg         *Config
	ds          *deviceState
	cf          *clientFactory
	Conn        *gws.Conn
	options     *gws.ClientOption
	hubVerified bool
}

func newClientFactory(cfg *Config, r *registry) *clientFactory {
	cf := &clientFactory{cfg: cfg, r: r, conns: map[string]*deviceClient{}}
	// parse pubkey
	if strings.TrimSpace(cfg.Hub.Key) != "" {
		pk, _, _, _, err := gossh.ParseAuthorizedKey([]byte(cfg.Hub.Key))
		if err == nil {
			cf.pubKey = pk
		} else {
			slog.Warn("invalid_hub_key", "err", err)
		}
	}
	cf.token = strings.TrimSpace(cfg.Hub.Token)
	if u, err := url.Parse(cfg.Hub.URL); err == nil {
		cf.url = u
	} else {
		slog.Warn("invalid_hub_url", "err", err)
	}
	return cf
}

func (cf *clientFactory) Notify(ip net.IP) {
	cf.mu.Lock()
	defer cf.mu.Unlock()
	key := ip.String()
	dc, ok := cf.conns[key]
	if !ok {
		if ds, ok2 := cf.r.get(ip); ok2 {
			dc = &deviceClient{ip: ip, cfg: cf.cfg, ds: ds, cf: cf}
			cf.conns[key] = dc
			go dc.connect()
		}
		return
	}
	// Do not send unsolicited data. The hub expects request/response.
	// Updates are stored in state and will be sent on the next GetData request.
	_ = dc // keep reference to avoid lint warnings
}

func (dc *deviceClient) getOptions() *gws.ClientOption {
	if dc.options != nil {
		return dc.options
	}
	if dc.cf.url == nil {
		return &gws.ClientOption{}
	}
	u := *dc.cf.url
	if u.Scheme == "https" {
		u.Scheme = "wss"
	} else {
		u.Scheme = "ws"
	}
	// ensure leading slash and preserve base path
	joined := path.Join(u.Path, "api/beszel/agent-connect")
	u.Path = "/" + strings.TrimPrefix(joined, "/")
	dc.options = &gws.ClientOption{
		Addr:      u.String(),
		TlsConfig: &tls.Config{InsecureSkipVerify: true},
		RequestHeader: http.Header{
			"User-Agent": []string{getUserAgent()},
			"X-Token":    []string{dc.cf.token},
			"X-Beszel":   []string{beszel.Version},
		},
	}
	return dc.options
}

// getUserAgent returns a browser-like UA to avoid proxy/CDN bot challenges.
func getUserAgent() string {
	const (
		uaBase    = "Mozilla/5.0 (%s) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36"
		uaWindows = "Windows NT 11.0; Win64; x64"
		uaMac     = "Macintosh; Intel Mac OS X 14_0_0"
	)
	if time.Now().Unix()%2 == 0 {
		return fmt.Sprintf(uaBase, uaWindows)
	}
	return fmt.Sprintf(uaBase, uaMac)
}

func (dc *deviceClient) connect() {
	opt := dc.getOptions()
	missing := []string{}
	if opt.Addr == "" { missing = append(missing, "hub url") }
	if dc.cf.token == "" { missing = append(missing, "token") }
	if dc.cf.pubKey == nil { missing = append(missing, "hub public key") }
	if len(missing) > 0 {
		slog.Warn("ws_not_configured", "missing", strings.Join(missing, ", "))
		return
	}
	if dc.Conn != nil {
		_ = dc.Conn.WriteClose(1000, nil)
	}
	slog.Info("ws_connect_attempt", "addr", opt.Addr, "ip", dc.ip.String())
	conn, _, err := gws.NewClient(dc, opt)
	if err != nil {
		slog.Warn("ws_connect_fail", "ip", dc.ip, "err", err)
		// retry later
		time.AfterFunc(5*time.Second, dc.connect)
		return
	}
	dc.Conn = conn
	slog.Info("ws_connected", "ip", dc.ip.String())
	go conn.ReadLoop()
}

func (dc *deviceClient) OnOpen(conn *gws.Conn) { conn.SetDeadline(time.Now().Add(70 * time.Second)) }

func (dc *deviceClient) OnClose(conn *gws.Conn, err error) {
	slog.Info("ws_closed", "ip", dc.ip, "err", strings.TrimPrefix(err.Error(), "gws: "))
	dc.hubVerified = false
	// reconnect with backoff
	time.AfterFunc(5*time.Second, dc.connect)
}

func (dc *deviceClient) OnPing(conn *gws.Conn, message []byte) { conn.SetDeadline(time.Now().Add(70 * time.Second)); conn.WritePong(message) }

func (dc *deviceClient) OnMessage(conn *gws.Conn, message *gws.Message) {
	defer message.Close()
	conn.SetDeadline(time.Now().Add(70 * time.Second))
	if message.Opcode != gws.OpcodeBinary {
		return
	}
	var req common.HubRequest[cbor.RawMessage]
	if err := cbor.NewDecoder(message.Data).Decode(&req); err != nil {
		slog.Debug("ws_msg_decode_err", "err", err)
		return
	}
	switch req.Action {
	case common.CheckFingerprint:
		var fr common.FingerprintRequest
		if err := cbor.Unmarshal(req.Data, &fr); err != nil { return }
		if err := dc.verifySignature(fr.Signature); err != nil { return }
		dc.hubVerified = true
		slog.Info("hub_verified", "ip", dc.ip.String())
		resp := &common.FingerprintResponse{Fingerprint: dc.ds.finger}
		if fr.NeedSysInfo { resp.Hostname = dc.ds.hostname }
		_ = dc.sendMessage(resp)
	case common.GetData:
		_ = dc.send()
	}
}

func (dc *deviceClient) verifySignature(signature []byte) error {
	sig := gossh.Signature{Format: dc.cf.pubKey.Type(), Blob: signature}
	if err := dc.cf.pubKey.Verify([]byte(dc.cf.token), &sig); err != nil {
		return errors.New("invalid signature")
	}
	return nil
}

func (dc *deviceClient) send() error {
	if dc.Conn == nil || !dc.hubVerified { return nil }
	dc.ds.mu.Lock()
	cd := dc.ds.buildCombinedData(beszel.Version)
	dc.ds.mu.Unlock()
	if len(cd.Stats.Temperatures) == 0 { return nil }
	err := dc.sendMessage(cd)
	if err == nil {
		slog.Info("ws_sent", "ip", dc.ip.String(), "temps", len(cd.Stats.Temperatures))
	}
	return err
}

func (dc *deviceClient) sendMessage(data any) error {
	bytes, err := cbor.Marshal(data)
	if err != nil { return err }
	return dc.Conn.WriteMessage(gws.OpcodeBinary, bytes)
}
