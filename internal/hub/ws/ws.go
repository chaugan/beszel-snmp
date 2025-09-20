package ws

import (
	"errors"
	"fmt"
	"time"
	"weak"

	"github.com/henrygd/beszel/internal/entities/container"
	"github.com/henrygd/beszel/internal/entities/system"

	"github.com/henrygd/beszel/internal/common"

	"github.com/fxamacker/cbor/v2"
	"github.com/lxzan/gws"
	"golang.org/x/crypto/ssh"
)

const (
	deadline = 70 * time.Second
)

// Handler implements the WebSocket event handler for agent connections.
type Handler struct {
	gws.BuiltinEventHandler
}

// WsConn represents a WebSocket connection to an agent.
type WsConn struct {
	conn         *gws.Conn
	responseChan chan *gws.Message
	DownChan     chan struct{}
}

// FingerprintRecord is fingerprints collection record data in the hub
type FingerprintRecord struct {
	Id          string `db:"id"`
	SystemId    string `db:"system"`
	Fingerprint string `db:"fingerprint"`
	Token       string `db:"token"`
}

var upgrader *gws.Upgrader

// GetUpgrader returns a singleton WebSocket upgrader instance.
func GetUpgrader() *gws.Upgrader {
	if upgrader != nil {
		return upgrader
	}
	handler := &Handler{}
	upgrader = gws.NewUpgrader(handler, &gws.ServerOption{})
	return upgrader
}

// NewWsConnection creates a new WebSocket connection wrapper.
func NewWsConnection(conn *gws.Conn) *WsConn {
	return &WsConn{
		conn:         conn,
		responseChan: make(chan *gws.Message, 1),
		DownChan:     make(chan struct{}, 1),
	}
}

// OnOpen sets a deadline for the WebSocket connection.
func (h *Handler) OnOpen(conn *gws.Conn) {
	conn.SetDeadline(time.Now().Add(deadline))
	fmt.Printf("[DEBUG] WebSocket connection opened: %s\n", conn.RemoteAddr())
}

// OnMessage routes incoming WebSocket messages to the response channel.
func (h *Handler) OnMessage(conn *gws.Conn, message *gws.Message) {
	conn.SetDeadline(time.Now().Add(deadline))
	fmt.Printf("[DEBUG] Received WebSocket message from %s: opcode=%d, length=%d\n",
		conn.RemoteAddr(), message.Opcode, message.Data.Len())

	if message.Opcode != gws.OpcodeBinary || message.Data.Len() == 0 {
		fmt.Printf("[DEBUG] Ignoring non-binary or empty message from %s\n", conn.RemoteAddr())
		return
	}

	wsConn, ok := conn.Session().Load("wsConn")
	if !ok {
		fmt.Printf("[DEBUG] No wsConn found in session for %s, closing connection\n", conn.RemoteAddr())
		_ = conn.WriteClose(1000, nil)
		return
	}

	fmt.Printf("[DEBUG] Attempting to route message to response channel for %s\n", conn.RemoteAddr())

	// Try to send message immediately
	select {
	case wsConn.(*WsConn).responseChan <- message:
		fmt.Printf("[DEBUG] Message successfully routed to response channel for %s\n", conn.RemoteAddr())
		return
	default:
		fmt.Printf("[DEBUG] No receiver ready for %s, starting buffering process\n", conn.RemoteAddr())
		// No receiver ready, try buffering with a timeout
		go func() {
			// Give the receiver a moment to be ready (up to 2 seconds)
			for i := 0; i < 20; i++ {
				time.Sleep(100 * time.Millisecond)
				select {
				case wsConn.(*WsConn).responseChan <- message:
					fmt.Printf("[DEBUG] Message successfully buffered and delivered to %s after %dms\n",
						conn.RemoteAddr(), (i+1)*100)
					return
				default:
					continue
				}
			}
			// If still no receiver after 2 seconds, close connection
			fmt.Printf("[DEBUG] No receiver found after 2 seconds for %s, closing connection\n", conn.RemoteAddr())
			wsConn.(*WsConn).Close(nil)
		}()
	}
}

// OnClose handles WebSocket connection closures and triggers system down status after delay.
func (h *Handler) OnClose(conn *gws.Conn, err error) {
	fmt.Printf("[DEBUG] WebSocket connection closed: %s, error: %v\n", conn.RemoteAddr(), err)
	wsConn, ok := conn.Session().Load("wsConn")
	if !ok {
		fmt.Printf("[DEBUG] No wsConn found in session during close for %s\n", conn.RemoteAddr())
		return
	}
	wsConn.(*WsConn).conn = nil
	// wait 5 seconds to allow reconnection before setting system down
	// use a weak pointer to avoid keeping references if the system is removed
	go func(downChan weak.Pointer[chan struct{}]) {
		time.Sleep(5 * time.Second)
		downChanValue := downChan.Value()
		if downChanValue != nil {
			*downChanValue <- struct{}{}
		}
	}(weak.Make(&wsConn.(*WsConn).DownChan))
}

// Close terminates the WebSocket connection gracefully.
func (ws *WsConn) Close(msg []byte) {
	if ws.IsConnected() {
		ws.conn.WriteClose(1000, msg)
	}
}

// Ping sends a ping frame to keep the connection alive.
func (ws *WsConn) Ping() error {
	ws.conn.SetDeadline(time.Now().Add(deadline))
	return ws.conn.WritePing(nil)
}

// sendMessage encodes data to CBOR and sends it as a binary message to the agent.
func (ws *WsConn) sendMessage(data common.HubRequest[any]) error {
	if ws.conn == nil {
		return gws.ErrConnClosed
	}
	bytes, err := cbor.Marshal(data)
	if err != nil {
		return err
	}
	return ws.conn.WriteMessage(gws.OpcodeBinary, bytes)
}

// RequestSystemData requests system metrics from the agent and unmarshals the response.
func (ws *WsConn) RequestSystemData(data *system.CombinedData) error {
	fmt.Printf("[DEBUG] RequestSystemData: Attempting to request system data\n")
	var message *gws.Message

	fmt.Printf("[DEBUG] RequestSystemData: Sending GetData request\n")
	err := ws.sendMessage(common.HubRequest[any]{
		Action: common.GetData,
	})
	if err != nil {
		fmt.Printf("[DEBUG] RequestSystemData: Failed to send GetData request: %v\n", err)
		return err
	}

	fmt.Printf("[DEBUG] RequestSystemData: Waiting for system data response...\n")
	select {
	case <-time.After(10 * time.Second):
		fmt.Printf("[DEBUG] RequestSystemData: Timeout waiting for system data response\n")
		ws.Close(nil)
		return gws.ErrConnClosed
	case message = <-ws.responseChan:
		fmt.Printf("[DEBUG] RequestSystemData: Received system data response (length: %d)\n", message.Data.Len())
	}
	defer message.Close()

	// Debug: Log raw CBOR data
	rawData := message.Data.Bytes()
	fmt.Printf("[DEBUG] RequestSystemData: Raw CBOR data length: %d\n", len(rawData))
	if len(rawData) > 0 {
		displayLen := 20
		if len(rawData) < displayLen {
			displayLen = len(rawData)
		}
		fmt.Printf("[DEBUG] RequestSystemData: First %d bytes: %x\n", displayLen, rawData[:displayLen])
	}

	err = cbor.Unmarshal(rawData, data)
	if err != nil {
		fmt.Printf("[DEBUG] RequestSystemData: Failed to unmarshal system data: %v\n", err)
		// Try to unmarshal as raw interface{} to see what we actually got
		var raw interface{}
		if cborErr := cbor.Unmarshal(rawData, &raw); cborErr == nil {
			fmt.Printf("[DEBUG] RequestSystemData: Raw unmarshaled data type: %T, value: %+v\n", raw, raw)
		}

		// Try backward compatibility: unmarshal as map and convert to struct
		fmt.Printf("[DEBUG] RequestSystemData: Attempting backward compatibility conversion\n")
		err = ws.convertMapToCombinedData(rawData, data)
		if err != nil {
			fmt.Printf("[DEBUG] RequestSystemData: Backward compatibility conversion failed: %v\n", err)
		} else {
			fmt.Printf("[DEBUG] RequestSystemData: Successfully converted using backward compatibility\n")
		}
	} else {
		fmt.Printf("[DEBUG] RequestSystemData: Successfully unmarshaled system data\n")
	}
	return err
}

// convertMapToCombinedData converts old format (map with numeric keys) to new format (struct)
func (ws *WsConn) convertMapToCombinedData(rawData []byte, data *system.CombinedData) error {
	// Unmarshal as map[interface{}]interface{}
	var rawMap map[interface{}]interface{}
	if err := cbor.Unmarshal(rawData, &rawMap); err != nil {
		return err
	}

	// Convert Stats (index 0)
	if statsMap, ok := rawMap[0].(map[interface{}]interface{}); ok {
		ws.convertMapToStats(statsMap, &data.Stats)
	}

	// Convert Info (index 1)
	if infoMap, ok := rawMap[1].(map[interface{}]interface{}); ok {
		ws.convertMapToInfo(infoMap, &data.Info)
	}

	// Convert Containers (index 2)
	if containersArray, ok := rawMap[2].([]interface{}); ok {
		data.Containers = make([]*container.Stats, len(containersArray))
		for i, containerMap := range containersArray {
			if containerMapData, ok := containerMap.(map[interface{}]interface{}); ok {
				containerStats := &container.Stats{}
				ws.convertMapToContainerStats(containerMapData, containerStats)
				data.Containers[i] = containerStats
			}
		}
	}

	return nil
}

// convertMapToStats converts map to Stats struct
func (ws *WsConn) convertMapToStats(statsMap map[interface{}]interface{}, stats *system.Stats) {
	if val, ok := statsMap[0]; ok {
		if f, ok := val.(float64); ok {
			stats.Cpu = f
		}
	}
	if val, ok := statsMap[2]; ok {
		if f, ok := val.(float64); ok {
			stats.Mem = f
		}
	}
	if val, ok := statsMap[3]; ok {
		if f, ok := val.(float64); ok {
			stats.MemUsed = f
		}
	}
	if val, ok := statsMap[4]; ok {
		if f, ok := val.(float64); ok {
			stats.MemPct = f
		}
	}
	if val, ok := statsMap[5]; ok {
		if f, ok := val.(float64); ok {
			stats.MemBuffCache = f
		}
	}
	if val, ok := statsMap[9]; ok {
		if f, ok := val.(float64); ok {
			stats.DiskTotal = f
		}
	}
	if val, ok := statsMap[10]; ok {
		if f, ok := val.(float64); ok {
			stats.DiskUsed = f
		}
	}
	if val, ok := statsMap[11]; ok {
		if f, ok := val.(float64); ok {
			stats.DiskPct = f
		}
	}
	if val, ok := statsMap[12]; ok {
		if f, ok := val.(float64); ok {
			stats.DiskReadPs = f
		}
	}
	if val, ok := statsMap[13]; ok {
		if f, ok := val.(float64); ok {
			stats.DiskWritePs = f
		}
	}
	if val, ok := statsMap[16]; ok {
		if f, ok := val.(float64); ok {
			stats.NetworkSent = f
		}
	}
	if val, ok := statsMap[17]; ok {
		if f, ok := val.(float64); ok {
			stats.NetworkRecv = f
		}
	}

	// Convert Temperatures (index 20)
	if tempMap, ok := statsMap[20].(map[interface{}]interface{}); ok {
		stats.Temperatures = make(map[string]float64)
		for key, val := range tempMap {
			if name, ok := key.(string); ok {
				if temp, ok := val.(float64); ok {
					stats.Temperatures[name] = temp
				}
			}
		}
	}

	// Convert Bandwidth (index 26)
	if bandwidthArray, ok := statsMap[26].([]interface{}); ok && len(bandwidthArray) == 2 {
		if sent, ok := bandwidthArray[0].(uint64); ok {
			stats.Bandwidth[0] = sent
		}
		if recv, ok := bandwidthArray[1].(uint64); ok {
			stats.Bandwidth[1] = recv
		}
	}

	// Convert LoadAvg (index 28)
	if loadArray, ok := statsMap[28].([]interface{}); ok && len(loadArray) == 3 {
		for i, val := range loadArray {
			if f, ok := val.(float64); ok && i < 3 {
				stats.LoadAvg[i] = f
			}
		}
	}
}

// convertMapToInfo converts map to Info struct
func (ws *WsConn) convertMapToInfo(infoMap map[interface{}]interface{}, info *system.Info) {
	if val, ok := infoMap[0]; ok {
		if s, ok := val.(string); ok {
			info.Hostname = s
		}
	}
	if val, ok := infoMap[1]; ok {
		if s, ok := val.(string); ok {
			info.KernelVersion = s
		}
	}
	if val, ok := infoMap[2]; ok {
		if i, ok := val.(int); ok {
			info.Cores = i
		}
	}
	if val, ok := infoMap[4]; ok {
		if s, ok := val.(string); ok {
			info.CpuModel = s
		}
	}
	if val, ok := infoMap[5]; ok {
		if u, ok := val.(uint64); ok {
			info.Uptime = u
		}
	}
	if val, ok := infoMap[6]; ok {
		if f, ok := val.(float64); ok {
			info.Cpu = f
		}
	}
	if val, ok := infoMap[7]; ok {
		if f, ok := val.(float64); ok {
			info.MemPct = f
		}
	}
	if val, ok := infoMap[8]; ok {
		if f, ok := val.(float64); ok {
			info.DiskPct = f
		}
	}
	if val, ok := infoMap[9]; ok {
		if f, ok := val.(float64); ok {
			info.Bandwidth = f
		}
	}
	if val, ok := infoMap[10]; ok {
		if s, ok := val.(string); ok {
			info.AgentVersion = s
		}
	}
	if val, ok := infoMap[13]; ok {
		if f, ok := val.(float64); ok {
			info.DashboardTemp = f
		}
	}
	if val, ok := infoMap[14]; ok {
		if o, ok := val.(uint8); ok {
			info.Os = system.Os(o)
		}
	}
	if val, ok := infoMap[15]; ok {
		if f, ok := val.(float64); ok {
			info.LoadAvg1 = f
		}
	}
	if val, ok := infoMap[16]; ok {
		if f, ok := val.(float64); ok {
			info.LoadAvg5 = f
		}
	}
	if val, ok := infoMap[17]; ok {
		if f, ok := val.(float64); ok {
			info.LoadAvg15 = f
		}
	}
	if val, ok := infoMap[18]; ok {
		if u, ok := val.(uint64); ok {
			info.BandwidthBytes = u
		}
	}

	// Convert LoadAvg array (index 19)
	if loadArray, ok := infoMap[19].([]interface{}); ok && len(loadArray) == 3 {
		for i, val := range loadArray {
			if f, ok := val.(float64); ok && i < 3 {
				info.LoadAvg[i] = f
			}
		}
	}
}

// convertMapToContainerStats converts map to container.Stats struct
func (ws *WsConn) convertMapToContainerStats(containerMap map[interface{}]interface{}, containerStats *container.Stats) {
	if val, ok := containerMap[0]; ok {
		if s, ok := val.(string); ok {
			containerStats.Name = s
		}
	}
	if val, ok := containerMap[1]; ok {
		if f, ok := val.(float64); ok {
			containerStats.Cpu = f
		}
	}
	if val, ok := containerMap[2]; ok {
		if f, ok := val.(float64); ok {
			containerStats.Mem = f
		}
	}
	if val, ok := containerMap[3]; ok {
		if f, ok := val.(float64); ok {
			containerStats.NetworkSent = f
		}
	}
	if val, ok := containerMap[4]; ok {
		if f, ok := val.(float64); ok {
			containerStats.NetworkRecv = f
		}
	}
}

// GetFingerprint authenticates with the agent using SSH signature and returns the agent's fingerprint.
// It supports both regular Beszel agents (with signature verification) and SNMP monitor agents (without signature verification).
func (ws *WsConn) GetFingerprint(token string, signer ssh.Signer, needSysInfo bool) (common.FingerprintResponse, error) {
	fmt.Printf("[DEBUG] GetFingerprint: Starting authentication with signature verification\n")
	var clientFingerprint common.FingerprintResponse
	challenge := []byte(token)

	signature, err := signer.Sign(nil, challenge)
	if err != nil {
		fmt.Printf("[DEBUG] GetFingerprint: Failed to create signature: %v\n", err)
		return clientFingerprint, err
	}

	fmt.Printf("[DEBUG] GetFingerprint: Sending CheckFingerprint request with signature (length: %d)\n", len(signature.Blob))
	// Try full signature verification first (for regular Beszel agents)
	err = ws.sendMessage(common.HubRequest[any]{
		Action: common.CheckFingerprint,
		Data: common.FingerprintRequest{
			Signature:   signature.Blob,
			NeedSysInfo: needSysInfo,
		},
	})
	if err != nil {
		fmt.Printf("[DEBUG] GetFingerprint: Failed to send message: %v\n", err)
		return clientFingerprint, err
	}

	fmt.Printf("[DEBUG] GetFingerprint: Waiting for response from agent...\n")
	var message *gws.Message
	select {
	case message = <-ws.responseChan:
		fmt.Printf("[DEBUG] GetFingerprint: Received response from agent\n")
	case <-time.After(5 * time.Second):
		fmt.Printf("[DEBUG] GetFingerprint: Timeout waiting for response, trying without signature verification\n")
		// If no response, try without signature verification (for SNMP monitor agents)
		return ws.GetFingerprintWithoutSignature(token, needSysInfo)
	}
	defer message.Close()

	err = cbor.Unmarshal(message.Data.Bytes(), &clientFingerprint)
	if err != nil {
		fmt.Printf("[DEBUG] GetFingerprint: Failed to unmarshal response: %v, trying without signature verification\n", err)
		// If signature verification failed, try without signature (for SNMP monitor agents)
		return ws.GetFingerprintWithoutSignature(token, needSysInfo)
	}

	fmt.Printf("[DEBUG] GetFingerprint: Successfully authenticated with signature verification, fingerprint: %s\n", clientFingerprint.Fingerprint)
	return clientFingerprint, err
}

// GetFingerprintWithoutSignature authenticates with SNMP monitor agents that skip signature verification.
func (ws *WsConn) GetFingerprintWithoutSignature(token string, needSysInfo bool) (common.FingerprintResponse, error) {
	fmt.Printf("[DEBUG] GetFingerprintWithoutSignature: Starting authentication without signature verification\n")
	var clientFingerprint common.FingerprintResponse

	fmt.Printf("[DEBUG] GetFingerprintWithoutSignature: Sending CheckFingerprint request without signature\n")
	// Send fingerprint request without signature (for SNMP monitor agents)
	err := ws.sendMessage(common.HubRequest[any]{
		Action: common.CheckFingerprint,
		Data: common.FingerprintRequest{
			Signature:   []byte{}, // Empty signature for SNMP agents
			NeedSysInfo: needSysInfo,
		},
	})
	if err != nil {
		fmt.Printf("[DEBUG] GetFingerprintWithoutSignature: Failed to send message: %v\n", err)
		return clientFingerprint, err
	}

	fmt.Printf("[DEBUG] GetFingerprintWithoutSignature: Waiting for response from agent...\n")
	var message *gws.Message
	select {
	case message = <-ws.responseChan:
		fmt.Printf("[DEBUG] GetFingerprintWithoutSignature: Received response from agent\n")
	case <-time.After(10 * time.Second):
		fmt.Printf("[DEBUG] GetFingerprintWithoutSignature: Timeout waiting for response\n")
		return clientFingerprint, errors.New("request expired")
	}
	defer message.Close()

	err = cbor.Unmarshal(message.Data.Bytes(), &clientFingerprint)
	if err != nil {
		fmt.Printf("[DEBUG] GetFingerprintWithoutSignature: Failed to unmarshal response: %v\n", err)
	} else {
		fmt.Printf("[DEBUG] GetFingerprintWithoutSignature: Successfully authenticated without signature verification, fingerprint: %s\n", clientFingerprint.Fingerprint)
	}
	return clientFingerprint, err
}

// IsConnected returns true if the WebSocket connection is active.
func (ws *WsConn) IsConnected() bool {
	return ws.conn != nil
}
