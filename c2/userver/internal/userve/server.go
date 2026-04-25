package userve

import (
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"unicode"
	"unicode/utf16"
)

const (
	cmdSendText          = 1
	cmdGetApps           = 2
	cmdConnectSession    = 3
	cmdOutputText        = 4
	cmdDisconnectSession = 5
	cmdPushFile          = 6
	cmdExecApp           = 7
	cmdEchoSend          = 8

	headerSize        = 3
	defaultHost       = "0.0.0.0"
	DefaultPort       = 8080
	maxOutputLog      = 200
	defaultOutputTail = 20
)

type packet struct {
	command byte
	payload []byte
}

type controlRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

type controlResponse struct {
	OK    bool     `json:"ok"`
	Lines []string `json:"lines,omitempty"`
}

type session struct {
	id             int
	conn           net.Conn
	addr           string
	ready          bool
	connectMessage string
	outputs        []string
	lastError      string
}

type sessionSnapshot struct {
	ID             int
	Addr           string
	Ready          bool
	ConnectMessage string
	LastError      string
	Active         bool
}

type sessionManager struct {
	mu       sync.Mutex
	sessions map[int]*session
	nextID   int
	activeID int
}

type controlService struct {
	sessions *sessionManager
	shutdown func()
}

func recvPacket(conn net.Conn) (*packet, error) {
	header := make([]byte, headerSize)
	if _, err := io.ReadFull(conn, header); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, io.EOF
		}
		return nil, err
	}

	payloadLength := int(binary.LittleEndian.Uint16(header[1:]))
	payload := make([]byte, payloadLength)
	if _, err := io.ReadFull(conn, payload); err != nil {
		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("connection closed during payload read, expected %d bytes", payloadLength)
		}
		return nil, err
	}

	return &packet{
		command: header[0],
		payload: payload,
	}, nil
}

func buildPacket(command byte, payload []byte) []byte {
	data := make([]byte, headerSize+len(payload))
	data[0] = command
	binary.LittleEndian.PutUint16(data[1:3], uint16(len(payload)))
	copy(data[3:], payload)
	return data
}

func (m *sessionManager) register(conn net.Conn) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sessions == nil {
		m.sessions = make(map[int]*session)
	}

	m.nextID++
	id := m.nextID
	m.sessions[id] = &session{
		id:      id,
		conn:    conn,
		addr:    conn.RemoteAddr().String(),
		outputs: make([]string, 0, maxOutputLog),
	}
	if m.activeID == 0 {
		m.activeID = id
	}

	return id
}

func (m *sessionManager) unregister(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.sessions, id)
	if m.activeID != id {
		return
	}

	m.activeID = 0
	for _, nextID := range m.sortedIDsLocked() {
		m.activeID = nextID
		break
	}
}

func (m *sessionManager) markReady(id int, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess := m.sessions[id]
	if sess == nil {
		return
	}

	sess.ready = true
	sess.connectMessage = message
}

func (m *sessionManager) addOutput(id int, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess := m.sessions[id]
	if sess == nil {
		return
	}

	if len(sess.outputs) == maxOutputLog {
		copy(sess.outputs, sess.outputs[1:])
		sess.outputs = sess.outputs[:maxOutputLog-1]
	}
	sess.outputs = append(sess.outputs, text)
}

func (m *sessionManager) setError(id int, text string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess := m.sessions[id]
	if sess == nil {
		return
	}

	sess.lastError = text
	if len(sess.outputs) == maxOutputLog {
		copy(sess.outputs, sess.outputs[1:])
		sess.outputs = sess.outputs[:maxOutputLog-1]
	}
	sess.outputs = append(sess.outputs, text)
}

func (m *sessionManager) setActive(id int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.sessions[id] == nil {
		return false
	}
	m.activeID = id
	return true
}

func (m *sessionManager) activeSnapshot() (sessionSnapshot, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess := m.sessions[m.activeID]
	if sess == nil {
		return sessionSnapshot{}, false
	}
	return snapshotFromSession(sess, true), true
}

func (m *sessionManager) snapshots() []sessionSnapshot {
	m.mu.Lock()
	defer m.mu.Unlock()

	ids := m.sortedIDsLocked()
	snaps := make([]sessionSnapshot, 0, len(ids))
	for _, id := range ids {
		snaps = append(snaps, snapshotFromSession(m.sessions[id], id == m.activeID))
	}
	return snaps
}

func (m *sessionManager) outputs(limit int) ([]string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	sess := m.sessions[m.activeID]
	if sess == nil {
		return nil, false
	}

	outputs := sess.outputs
	if limit > 0 && limit < len(outputs) {
		outputs = outputs[len(outputs)-limit:]
	}

	cloned := make([]string, len(outputs))
	copy(cloned, outputs)
	return cloned, true
}

func (m *sessionManager) sendPacket(command byte, payload []byte) (bool, string) {
	m.mu.Lock()
	sess := m.sessions[m.activeID]
	m.mu.Unlock()

	if sess == nil {
		return false, "ERR no active connection"
	}

	if _, err := sess.conn.Write(buildPacket(command, payload)); err != nil {
		m.unregister(sess.id)
		return false, fmt.Sprintf("ERR connection %d lost: %v", sess.id, err)
	}

	return true, fmt.Sprintf("OK sent command %d to connection %d", command, sess.id)
}

func (m *sessionManager) closeAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, sess := range m.sessions {
		_ = sess.conn.Close()
	}
}

func (m *sessionManager) sortedIDsLocked() []int {
	ids := make([]int, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}

func snapshotFromSession(sess *session, active bool) sessionSnapshot {
	return sessionSnapshot{
		ID:             sess.id,
		Addr:           sess.addr,
		Ready:          sess.ready,
		ConnectMessage: sess.connectMessage,
		LastError:      sess.lastError,
		Active:         active,
	}
}

func (c *controlService) printAsync(format string, args ...any) {
	fmt.Printf(format, args...)
}

func (c *controlService) statusLines() []string {
	snap, ok := c.sessions.activeSnapshot()
	if !ok {
		return []string{"No active connection."}
	}

	lines := []string{
		fmt.Sprintf("Active connection: #%d", snap.ID),
		fmt.Sprintf("Address: %s", snap.Addr),
		fmt.Sprintf("Ready: %t", snap.Ready),
	}
	if snap.ConnectMessage != "" {
		lines = append(lines, fmt.Sprintf("Connect message: %s", snap.ConnectMessage))
	}
	if snap.LastError != "" {
		lines = append(lines, fmt.Sprintf("Last error: %s", snap.LastError))
	}
	return lines
}

func (c *controlService) listLines() []string {
	snaps := c.sessions.snapshots()
	if len(snaps) == 0 {
		return []string{"No active connections."}
	}

	lines := make([]string, 0, len(snaps))
	for _, snap := range snaps {
		marker := " "
		if snap.Active {
			marker = "*"
		}
		line := fmt.Sprintf("%s #%d %s ready=%t", marker, snap.ID, snap.Addr, snap.Ready)
		if snap.ConnectMessage != "" {
			line += fmt.Sprintf(" msg=%q", snap.ConnectMessage)
		}
		lines = append(lines, line)
	}
	return lines
}

func (c *controlService) outputsLines(limit int) []string {
	outputs, ok := c.sessions.outputs(limit)
	if !ok {
		return []string{"ERR no active connection"}
	}
	if len(outputs) == 0 {
		return []string{"No buffered output."}
	}
	return outputs
}

func (c *controlService) sendCommand(text string) (bool, string) {
	cleaned := strings.TrimSpace(text)
	if cleaned == "" {
		return false, "ERR empty command"
	}
	if !isASCII(cleaned) {
		return false, "ERR commands must be ASCII"
	}
	return c.sessions.sendPacket(cmdEchoSend, []byte(cleaned))
}

func (c *controlService) getApps() (bool, string) {
	return c.sessions.sendPacket(cmdGetApps, nil)
}

func (c *controlService) disconnect() (bool, string) {
	return c.sessions.sendPacket(cmdDisconnectSession, nil)
}

func (c *controlService) pushFile(path string) (bool, string) {
	filename := filepath.Base(path)
	if filename == "" {
		return false, "ERR missing filename"
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return false, fmt.Sprintf("ERR failed to read %s: %v", path, err)
	}
	if !isASCII(filename) {
		return false, "ERR filename must be ASCII"
	}
	if len(filename) > 255 {
		return false, "ERR filename too long"
	}

	payload := make([]byte, 1+len(filename)+len(data))
	payload[0] = byte(len(filename))
	copy(payload[1:], []byte(filename))
	copy(payload[1+len(filename):], data)

	ok, response := c.sessions.sendPacket(cmdPushFile, payload)
	if !ok {
		return false, response
	}
	return true, fmt.Sprintf("Pushed %s to active EFI system.", filename)
}

func (c *controlService) runApp() (bool, string) {
	return c.sessions.sendPacket(cmdExecApp, nil)
}

func (c *controlService) handleControlRequest(req controlRequest) controlResponse {
	command := strings.TrimSpace(req.Command)
	switch command {
	case "list":
		return controlResponse{OK: true, Lines: c.listLines()}
	case "status":
		return controlResponse{OK: true, Lines: c.statusLines()}
	case "use":
		if len(req.Args) != 1 {
			return controlResponse{OK: false, Lines: []string{"ERR use requires exactly one connection id"}}
		}
		id, err := strconv.Atoi(req.Args[0])
		if err != nil {
			return controlResponse{OK: false, Lines: []string{fmt.Sprintf("ERR invalid connection id: %s", req.Args[0])}}
		}
		if !c.sessions.setActive(id) {
			return controlResponse{OK: false, Lines: []string{fmt.Sprintf("ERR unknown connection id %d", id)}}
		}
		return controlResponse{OK: true, Lines: []string{fmt.Sprintf("Active connection set to #%d", id)}}
	case "outputs":
		limit := defaultOutputTail
		if len(req.Args) > 1 {
			return controlResponse{OK: false, Lines: []string{"ERR outputs accepts at most one limit"}}
		}
		if len(req.Args) == 1 {
			value, err := strconv.Atoi(req.Args[0])
			if err != nil || value < 0 {
				return controlResponse{OK: false, Lines: []string{fmt.Sprintf("ERR invalid output limit: %s", req.Args[0])}}
			}
			limit = value
		}
		return controlResponse{OK: true, Lines: c.outputsLines(limit)}
	case "apps":
		return responseFromResult(c.getApps())
	case "disconnect":
		return responseFromResult(c.disconnect())
	case "push":
		if len(req.Args) != 1 {
			return controlResponse{OK: false, Lines: []string{"ERR push requires a file path"}}
		}
		return responseFromResult(c.pushFile(req.Args[0]))
	case "run":
		return responseFromResult(c.runApp())
	case "echo", "raw":
		return responseFromResult(c.sendCommand(strings.Join(req.Args, " ")))
	case "stop":
		if c.shutdown != nil {
			c.shutdown()
		}
		return controlResponse{OK: true, Lines: []string{"Service shutting down."}}
	default:
		return controlResponse{OK: false, Lines: []string{fmt.Sprintf("ERR unknown control command %q", command)}}
	}
}

func responseFromResult(ok bool, line string) controlResponse {
	return controlResponse{OK: ok, Lines: []string{line}}
}

func decodeRemoteTextPayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}

	if text, ok := decodeUTF16Payload(payload); ok {
		return text
	}

	if isASCIIBytes(payload) {
		return string(payload)
	}

	return "hex:" + strings.ToUpper(hex.EncodeToString(payload))
}

func decodeUTF16Payload(payload []byte) (string, bool) {
	if len(payload)%2 != 0 {
		return "", false
	}

	words := make([]uint16, len(payload)/2)
	for i := 0; i < len(words); i++ {
		words[i] = binary.LittleEndian.Uint16(payload[i*2:])
	}

	text := string(utf16.Decode(words))
	if !looksReadableText(text) {
		return "", false
	}

	return text, true
}

func isASCIIBytes(payload []byte) bool {
	for _, b := range payload {
		if b > 127 {
			return false
		}
	}
	return true
}

func looksReadableText(text string) bool {
	if text == "" {
		return true
	}

	for _, r := range text {
		switch {
		case r == '\n', r == '\r', r == '\t':
			continue
		case unicode.IsPrint(r):
			continue
		default:
			return false
		}
	}

	return true
}

func isASCII(text string) bool {
	for _, r := range text {
		if r > 127 {
			return false
		}
	}
	return true
}

func handleConnection(conn net.Conn, service *controlService) {
	defer conn.Close()

	id := service.sessions.register(conn)
	host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		host = conn.RemoteAddr().String()
	}
	service.printAsync("[!] Connection #%d connected from %s\n", id, host)

	defer func() {
		service.sessions.unregister(id)
		service.printAsync("[!] Connection #%d disconnected.\n", id)
	}()

	for {
		pkt, err := recvPacket(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				service.sessions.setError(id, "ERR "+err.Error())
			}
			return
		}

		switch pkt.command {
		case cmdConnectSession:
			msg := decodeRemoteTextPayload(pkt.payload)
			service.sessions.markReady(id, msg)
			service.printAsync("[+] Connection #%d ready: %s\n", id, msg)
		case cmdOutputText:
			text := decodeRemoteTextPayload(pkt.payload)
			service.sessions.addOutput(id, text)
			service.printAsync("[OUTPUT #%d] %s\n", id, text)
		case cmdDisconnectSession:
			return
		}
	}
}

func defaultControlSocketDir() string {
	cacheDir, err := os.UserCacheDir()
	if err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "userve")
	}
	return filepath.Join(os.TempDir(), "userve")
}

func DefaultControlSocketPath() string {
	return filepath.Join(defaultControlSocketDir(), "control.sock")
}

func listenControlSocket(path string) (net.Listener, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("failed to create control socket directory: %w", err)
	}

	if info, err := os.Stat(path); err == nil {
		if info.IsDir() {
			return nil, fmt.Errorf("control socket path is a directory: %s", path)
		}
		conn, dialErr := net.Dial("unix", path)
		if dialErr == nil {
			_ = conn.Close()
			return nil, fmt.Errorf("control socket already in use: %s", path)
		}
		if removeErr := os.Remove(path); removeErr != nil {
			return nil, fmt.Errorf("failed to remove stale control socket: %w", removeErr)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("failed to inspect control socket path: %w", err)
	}

	listener, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	if chmodErr := os.Chmod(path, 0o600); chmodErr != nil {
		_ = listener.Close()
		_ = os.Remove(path)
		return nil, fmt.Errorf("failed to secure control socket: %w", chmodErr)
	}
	return listener, nil
}

func handleControlConn(conn net.Conn, service *controlService) {
	defer conn.Close()

	var req controlRequest
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		_ = json.NewEncoder(conn).Encode(controlResponse{
			OK:    false,
			Lines: []string{fmt.Sprintf("ERR invalid control request: %v", err)},
		})
		return
	}

	resp := service.handleControlRequest(req)
	_ = json.NewEncoder(conn).Encode(resp)
}

func RunService(port int, controlSocket string) error {
	sessionManager := &sessionManager{
		sessions: make(map[int]*session),
	}

	stop := make(chan struct{})
	var stopOnce sync.Once
	shutdown := func() {
		stopOnce.Do(func() { close(stop) })
	}

	service := &controlService{
		sessions: sessionManager,
		shutdown: shutdown,
	}

	tcpListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", defaultHost, port))
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}
	defer tcpListener.Close()

	controlListener, err := listenControlSocket(controlSocket)
	if err != nil {
		return fmt.Errorf("failed to start control socket: %w", err)
	}
	defer func() {
		_ = controlListener.Close()
		_ = os.Remove(controlSocket)
	}()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalChan)

	go func() {
		select {
		case <-signalChan:
			shutdown()
		case <-stop:
		}
		_ = tcpListener.Close()
		_ = controlListener.Close()
		sessionManager.closeAll()
	}()

	go func() {
		for {
			conn, err := tcpListener.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					fmt.Fprintf(os.Stderr, "accept error: %v\n", err)
					continue
				}
			}
			go handleConnection(conn, service)
		}
	}()

	go func() {
		for {
			conn, err := controlListener.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					fmt.Fprintf(os.Stderr, "control accept error: %v\n", err)
					continue
				}
			}
			go handleControlConn(conn, service)
		}
	}()

	fmt.Printf("Listening for UEFI connections on port %d...\n", port)
	fmt.Printf("Listening for local control on %s\n", controlSocket)

	<-stop
	return nil
}
