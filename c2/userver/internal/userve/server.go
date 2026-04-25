package userve

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"userve/internal/commands"
	"userve/internal/control"
	"userve/internal/hub"
	"userve/internal/stream"
)

const (
	defaultHost = "0.0.0.0"
	DefaultPort = 8080
)

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

func RunService(port int, controlSocket string) error {
	h := hub.New()
	console := stream.NewConsoleSink(os.Stdout)
	tail := stream.NewTailSink(20)

	unsubscribeConsole := h.Subscribe(console.Handle)
	defer unsubscribeConsole()
	unsubscribeTail := h.Subscribe(tail.Handle)
	defer unsubscribeTail()

	stop := make(chan struct{})
	var stopOnce sync.Once
	shutdown := func() {
		stopOnce.Do(func() {
			close(stop)
		})
	}

	commandService := commands.New(h)
	controlService := control.NewService(h, commandService, tail, shutdown)

	tcpListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", defaultHost, port))
	if err != nil {
		return fmt.Errorf("failed to start TCP server: %w", err)
	}
	defer tcpListener.Close()

	controlListener, err := control.ListenSocket(controlSocket)
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
		h.Close()
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
			if rejectErr := h.Accept(conn); rejectErr != nil {
				fmt.Fprintf(os.Stderr, "connection rejected from %s: %v\n", conn.RemoteAddr(), rejectErr)
				_ = conn.Close()
			}
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
			go control.HandleConn(conn, controlService)
		}
	}()

	fmt.Printf("Listening for UEFI connections on port %d...\n", port)
	fmt.Printf("Listening for local control on %s\n", controlSocket)

	<-stop
	return nil
}
