package userve

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/chzyer/readline"
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

	headerSize   = 3
	defaultHost  = "0.0.0.0"
	DefaultPort  = 8080
	maxOutputLog = 200
	shellPrompt  = "userve> "
)

type packet struct {
	command byte
	payload []byte
}

//	func recvExactly(r io.Reader, size int) ([]byte, error) {
//		buf := make([]byte, size)
//		_, err := io.ReadFull(r, buf)
//		if err != nil {
//			return nil, err
//		}
//		return buf, nil
//	}
//
//	func recvPacket(conn net.Conn) (*packet, error) {
//		header, err := recvExactly(conn, headerSize)
//		if err != nil {
//			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
//				return nil, io.EOF
//			}
//			return nil, err
//		}
//		println("Received packet header")
//
//		payloadLength := int(binary.LittleEndian.Uint16(header[1:]))
//		payload, err := recvExactly(conn, payloadLength)
//		if err != nil {
//			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
//				return nil, fmt.Errorf("connection closed during payload read, expected %d bytes", payloadLength)
//			}
//			return nil, err
//		}
//
//		return &packet{
//			command: header[0],
//			payload: payload,
//		}, nil
//	}

func recvPacket(conn net.Conn) (*packet, error) {
	// make a full read of the length-prefixed packet, handling partial reads
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

type session struct {
	conn           net.Conn
	addr           string
	ready          bool
	connectMessage string
	outputs        []string
	lastError      string
}

type sessionManager struct {
	mu      sync.Mutex
	session *session
}

func (m *sessionManager) register(conn net.Conn) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.session = &session{
		conn:    conn,
		addr:    conn.RemoteAddr().String(),
		outputs: make([]string, 0, maxOutputLog),
	}

	return 1
}

func (m *sessionManager) unregister() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.session = nil
	return true
}

func (m *sessionManager) isConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.session != nil
}

func (m *sessionManager) markReady(message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session == nil {
		return
	}
	m.session.ready = true
	m.session.connectMessage = message
}

func (m *sessionManager) addOutput(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session == nil {
		return
	}
	if len(m.session.outputs) == maxOutputLog {
		copy(m.session.outputs, m.session.outputs[1:])
		m.session.outputs = m.session.outputs[:maxOutputLog-1]
	}
	m.session.outputs = append(m.session.outputs, text)
}

func (m *sessionManager) setError(text string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.session == nil {
		return
	}
	m.session.lastError = text
	if len(m.session.outputs) == maxOutputLog {
		copy(m.session.outputs, m.session.outputs[1:])
		m.session.outputs = m.session.outputs[:maxOutputLog-1]
	}
	m.session.outputs = append(m.session.outputs, text)
}

func (m *sessionManager) getStatus() map[string]any {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return map[string]any{"connected": false}
	}

	return map[string]any{
		"connected":       true,
		"address":         m.session.addr,
		"ready":           m.session.ready,
		"connect_message": m.session.connectMessage,
	}
}

func (m *sessionManager) getOutputs(limit int) (bool, any) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.session == nil {
		return false, "ERR no system connected"
	}

	outputs := m.session.outputs
	if limit > 0 && limit < len(outputs) {
		outputs = outputs[len(outputs)-limit:]
	}

	result := make([]string, len(outputs))
	copy(result, outputs)
	return true, map[string]any{"outputs": result}
}

func (m *sessionManager) sendPacket(command byte, payload []byte) (bool, string) {
	m.mu.Lock()
	if m.session == nil {
		m.mu.Unlock()
		return false, "ERR no system connected"
	}
	conn := m.session.conn
	m.mu.Unlock()

	if _, err := conn.Write(buildPacket(command, payload)); err != nil {
		m.unregister()
		return false, fmt.Sprintf("ERR connection lost: %v", err)
	}

	return true, fmt.Sprintf("OK sent command %d", command)
}

type controlService struct {
	sessions  *sessionManager
	console   *readline.Instance
	consoleMu sync.Mutex
}

func (c *controlService) status() map[string]any {
	return c.sessions.getStatus()
}

func (c *controlService) attachConsole(rl *readline.Instance) {
	c.consoleMu.Lock()
	defer c.consoleMu.Unlock()
	c.console = rl
}

func (c *controlService) detachConsole() {
	c.consoleMu.Lock()
	defer c.consoleMu.Unlock()
	c.console = nil
}

func (c *controlService) printAsync(format string, args ...any) {
	c.consoleMu.Lock()
	defer c.consoleMu.Unlock()

	if c.console != nil {
		fmt.Fprintf(c.console.Stdout(), format, args...)
		c.console.Refresh()
		return
	}

	fmt.Printf(format, args...)
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

func (c *controlService) outputs(limit int) (bool, any) {
	return c.sessions.getOutputs(limit)
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

	service.sessions.register(conn)
	host, _, err := net.SplitHostPort(conn.RemoteAddr().String())
	if err != nil {
		host = conn.RemoteAddr().String()
	}
	service.printAsync("\n[!] System connected from %s\n", host)

	defer func() {
		service.sessions.unregister()
		service.printAsync("\n[!] System disconnected.\n")
	}()

	for {
		pkt, err := recvPacket(conn)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				service.sessions.setError("ERR " + err.Error())
			}
			return
		}
		switch pkt.command {
		case cmdConnectSession:
			msg := string(pkt.payload)
			service.sessions.markReady(msg)
			service.printAsync("[+] Session ready: %s\n", msg)
		case cmdOutputText:
			text := string(pkt.payload)
			service.sessions.addOutput(text)
			service.printAsync("[OUTPUT] %s\n", text)
		case cmdDisconnectSession:
			return
		}
	}
}

func printShellHelp() {
	fmt.Println("Shell ready for EFI control.")
	fmt.Println("Type a raw command to send it directly to the connected EFI client.")
	fmt.Println("Built-in commands:")
	fmt.Println("  help         Show this help text")
	fmt.Println("  status       Show session status and remote address")
	fmt.Println("  apps         Ask the client for its app list")
	fmt.Println("  disconnect   Tell the client to disconnect")
	fmt.Println("  push <file>  Upload a local file to the client")
	fmt.Println("  run          Ask the client to execute the selected app")
	fmt.Println("  echo <text>  Send ASCII text to the connected client")
	fmt.Println("  exit | quit  Stop the local server")
	fmt.Println("Tab completion is available for the built-in commands.")
}

func newShellCompleter() *readline.PrefixCompleter {
	return readline.NewPrefixCompleter(
		readline.PcItem("help"),
		readline.PcItem("?"),
		readline.PcItem("status"),
		readline.PcItem("apps"),
		readline.PcItem("disconnect"),
		readline.PcItem("push"),
		readline.PcItem("run"),
		readline.PcItem("echo"),
		readline.PcItem("exit"),
		readline.PcItem("quit"),
	)
}

func shellHistoryPath() string {
	cacheDir, err := os.UserCacheDir()
	if err == nil && cacheDir != "" {
		return filepath.Join(cacheDir, "userve", "shell.history")
	}
	return filepath.Join(os.TempDir(), "userve-shell.history")
}

func operatorConsoleLoop(service *controlService, stop chan struct{}, stopOnce *sync.Once) {
	historyPath := shellHistoryPath()
	_ = os.MkdirAll(filepath.Dir(historyPath), 0o755)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          shellPrompt,
		HistoryFile:     historyPath,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		AutoComplete:    newShellCompleter(),
	})
	if err != nil {
		fmt.Printf("failed to initialize interactive shell: %v\n", err)
		return
	}
	defer rl.Close()
	service.attachConsole(rl)
	defer service.detachConsole()

	printShellHelp()
	for {
		select {
		case <-stop:
			return
		default:
		}

		line, err := rl.Readline()
		if errors.Is(err, readline.ErrInterrupt) {
			if strings.TrimSpace(line) == "" {
				fmt.Println()
				continue
			}
		}
		if errors.Is(err, io.EOF) {
			stopOnce.Do(func() { close(stop) })
			return
		}
		if err != nil {
			fmt.Printf("shell error: %v\n", err)
			continue
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		switch {
		case line == "help" || line == "?":
			printShellHelp()
		case line == "exit" || line == "quit" || line == "q":
			fmt.Println("Shutting down...")
			stopOnce.Do(func() { close(stop) })
			return
		case line == "status":
			fmt.Println(service.status())
		case line == "apps":
			_, response := service.getApps()
			fmt.Println(response)
		case line == "disconnect":
			_, response := service.disconnect()
			fmt.Println(response)
		case strings.HasPrefix(line, "push "):
			filename := strings.TrimSpace(strings.TrimPrefix(line, "push "))
			if filename == "" {
				fmt.Println("Error: missing filename")
				continue
			}
			data, err := os.ReadFile(filename)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}
			if !isASCII(filename) {
				fmt.Println("Error: filename must be ASCII")
				continue
			}
			if len(filename) > 255 {
				fmt.Println("Error: filename too long")
				continue
			}

			payload := make([]byte, 1+len(filename)+len(data))
			payload[0] = byte(len(filename))
			copy(payload[1:], []byte(filename))
			copy(payload[1+len(filename):], data)

			ok, response := service.sessions.sendPacket(cmdPushFile, payload)
			if !ok {
				fmt.Println(response)
				continue
			}
			fmt.Printf("Pushed %s to EFI system.\n", filename)
		case line == "run":
			if !service.sessions.isConnected() {
				fmt.Println("ERR: No system connected.")
				continue
			}
			fmt.Println("remote execution mode")
			ok, response := service.sessions.sendPacket(cmdExecApp, nil)
			if !ok {
				fmt.Println(response)
			}
		case strings.HasPrefix(line, "echo "):
			text := strings.TrimSpace(strings.TrimPrefix(line, "echo "))
			if text == "" {
				fmt.Println("ERR empty command")
				continue
			}
			ok, response := service.sendCommand(text)
			if !ok {
				fmt.Println(response)
			}
		default:
			ok, response := service.sendCommand(line)
			if !ok {
				fmt.Println(response)
			}
		}
	}
}

func Run(port int) error {
	sessionManager := &sessionManager{}
	controlService := &controlService{sessions: sessionManager}

	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", defaultHost, port))
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}
	defer listener.Close()

	stop := make(chan struct{})
	var stopOnce sync.Once

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(signalChan)

	go func() {
		select {
		case <-signalChan:
			stopOnce.Do(func() { close(stop) })
			_ = listener.Close()
		case <-stop:
			_ = listener.Close()
		}
	}()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				select {
				case <-stop:
					return
				default:
					fmt.Fprintf(os.Stderr, "accept error: %v\n", err)
					continue
				}
			}
			go handleConnection(conn, controlService)
		}
	}()

	fmt.Printf("Listening for connection on port %d...\n", port)
	operatorConsoleLoop(controlService, stop, &stopOnce)
	return nil
}
