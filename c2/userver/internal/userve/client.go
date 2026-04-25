package userve

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/chzyer/readline"

	"userve/internal/control"
)

const shellPrompt = "userve> "

const (
	runPollInterval = 200 * time.Millisecond
	runIdleTimeout  = 2 * time.Second
	runStartTimeout = 3 * time.Second
)

func sendControlRequest(controlSocket string, req control.ControlRequest) (control.ControlResponse, error) {
	conn, err := net.Dial("unix", controlSocket)
	if err != nil {
		return control.ControlResponse{}, fmt.Errorf("failed to connect to local control socket %s: %w", controlSocket, err)
	}
	defer conn.Close()

	if err := json.NewEncoder(conn).Encode(req); err != nil {
		return control.ControlResponse{}, fmt.Errorf("failed to send control request: %w", err)
	}

	var resp control.ControlResponse
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return control.ControlResponse{}, fmt.Errorf("failed to read control response: %w", err)
	}
	return resp, nil
}

func printControlResponse(resp control.ControlResponse) {
	for _, line := range resp.Lines {
		fmt.Println(line)
	}
}

func RunControlCommand(controlSocket, command string, args []string) error {
	if command == "run" {
		return runAndFollow(controlSocket)
	}

	req, err := requestFromCommand(command, args)
	if err != nil {
		return err
	}

	resp, err := sendControlRequest(controlSocket, req)
	if err != nil {
		return err
	}

	printControlResponse(resp)
	if !resp.OK {
		return fmt.Errorf("control command failed")
	}
	return nil
}

func RunShell(controlSocket string) error {
	historyPath := shellHistoryPath()
	_ = os.MkdirAll(defaultControlSocketDir(), 0o700)

	rl, err := readline.NewEx(&readline.Config{
		Prompt:          shellPrompt,
		HistoryFile:     historyPath,
		InterruptPrompt: "^C",
		EOFPrompt:       "exit",
		AutoComplete:    newShellCompleter(),
	})
	if err != nil {
		return fmt.Errorf("failed to initialize interactive shell: %w", err)
	}
	defer rl.Close()

	printShellHelp(controlSocket)
	for {
		line, err := rl.Readline()
		if errors.Is(err, readline.ErrInterrupt) {
			if strings.TrimSpace(line) == "" {
				fmt.Println()
				continue
			}
		}
		if errors.Is(err, io.EOF) {
			return nil
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
			printShellHelp(controlSocket)
			continue
		case line == "exit" || line == "quit" || line == "q":
			return nil
		case line == "run":
			if err := runAndFollow(controlSocket); err != nil {
				fmt.Println(err)
			}
			continue
		}

		req := requestFromLine(line)
		resp, err := sendControlRequest(controlSocket, req)
		if err != nil {
			fmt.Println(err)
			continue
		}
		printControlResponse(resp)
	}
}

func requestFromCommand(command string, args []string) (control.ControlRequest, error) {
	switch command {
	case "list", "status", "apps", "disconnect", "run", "stop":
		return control.ControlRequest{Command: command, Args: args}, nil
	case "use", "push":
		if len(args) != 1 {
			return control.ControlRequest{}, fmt.Errorf("%s requires exactly one argument", command)
		}
		return control.ControlRequest{Command: command, Args: args}, nil
	case "outputs":
		if len(args) > 1 {
			return control.ControlRequest{}, fmt.Errorf("outputs accepts at most one argument")
		}
		return control.ControlRequest{Command: command, Args: args}, nil
	case "echo", "raw":
		if len(args) == 0 {
			return control.ControlRequest{}, fmt.Errorf("%s requires text to send", command)
		}
		return control.ControlRequest{Command: command, Args: []string{strings.Join(args, " ")}}, nil
	default:
		return control.ControlRequest{}, fmt.Errorf("unknown command %q", command)
	}
}

func requestFromLine(line string) control.ControlRequest {
	switch {
	case line == "list", line == "status", line == "apps", line == "disconnect", line == "run", line == "stop":
		return control.ControlRequest{Command: line}
	case strings.HasPrefix(line, "use "):
		return control.ControlRequest{Command: "use", Args: []string{strings.TrimSpace(strings.TrimPrefix(line, "use "))}}
	case strings.HasPrefix(line, "push "):
		return control.ControlRequest{Command: "push", Args: []string{strings.TrimSpace(strings.TrimPrefix(line, "push "))}}
	case strings.HasPrefix(line, "outputs "):
		return control.ControlRequest{Command: "outputs", Args: []string{strings.TrimSpace(strings.TrimPrefix(line, "outputs "))}}
	case line == "outputs":
		return control.ControlRequest{Command: "outputs"}
	case strings.HasPrefix(line, "echo "):
		return control.ControlRequest{Command: "echo", Args: []string{strings.TrimSpace(strings.TrimPrefix(line, "echo "))}}
	default:
		return control.ControlRequest{Command: "raw", Args: []string{line}}
	}
}

func fetchAllOutputs(controlSocket string) ([]string, error) {
	resp, err := sendControlRequest(controlSocket, control.ControlRequest{
		Command: "outputs",
		Args:    []string{"0"},
	})
	if err != nil {
		return nil, err
	}
	if !resp.OK {
		return nil, fmt.Errorf("%s", strings.Join(resp.Lines, "\n"))
	}
	if len(resp.Lines) == 1 && resp.Lines[0] == "No buffered output." {
		return []string{}, nil
	}
	return resp.Lines, nil
}

func runAndFollow(controlSocket string) error {
	baseline, err := fetchAllOutputs(controlSocket)
	if err != nil {
		baseline = nil
	}

	resp, err := sendControlRequest(controlSocket, control.ControlRequest{Command: "run"})
	if err != nil {
		return err
	}

	printControlResponse(resp)
	if !resp.OK {
		return fmt.Errorf("control command failed")
	}

	fmt.Println("Streaming remote output. Press Ctrl+C to stop.")
	return followOutputs(controlSocket, len(baseline))
}

func followOutputs(controlSocket string, seen int) error {
	ticker := time.NewTicker(runPollInterval)
	defer ticker.Stop()

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(signalChan)

	startedAt := time.Now()
	lastOutputAt := time.Time{}

	for {
		select {
		case <-signalChan:
			fmt.Println()
			return nil
		case <-ticker.C:
			lines, err := fetchAllOutputs(controlSocket)
			if err != nil {
				return err
			}

			if len(lines) < seen {
				seen = 0
			}
			if len(lines) > seen {
				for _, line := range lines[seen:] {
					fmt.Println(line)
				}
				seen = len(lines)
				lastOutputAt = time.Now()
				continue
			}

			now := time.Now()
			if lastOutputAt.IsZero() {
				if now.Sub(startedAt) >= runStartTimeout {
					return nil
				}
				continue
			}
			if now.Sub(lastOutputAt) >= runIdleTimeout {
				return nil
			}
		}
	}
}

func printShellHelp(controlSocket string) {
	fmt.Println("Local control shell ready.")
	fmt.Printf("Control socket: %s\n", controlSocket)
	fmt.Println("Commands:")
	fmt.Println("  list           List remote connections")
	fmt.Println("  status         Show the active connection")
	fmt.Println("  use <id>       Select the active connection")
	fmt.Println("  outputs [n]    Show buffered output from the active connection")
	fmt.Println("  apps           Ask the active client for its app list")
	fmt.Println("  push <file>    Upload a local file to the active client")
	fmt.Println("  run            Ask the active client to execute the selected app")
	fmt.Println("  disconnect     Tell the active client to disconnect")
	fmt.Println("  echo <text>    Send ASCII text to the active client")
	fmt.Println("  stop           Stop the background service")
	fmt.Println("  exit | quit    Leave the local control shell")
	fmt.Println("Any other line is sent as a raw command to the active client.")
}

func newShellCompleter() *readline.PrefixCompleter {
	return readline.NewPrefixCompleter(
		readline.PcItem("help"),
		readline.PcItem("?"),
		readline.PcItem("list"),
		readline.PcItem("status"),
		readline.PcItem("use"),
		readline.PcItem("outputs"),
		readline.PcItem("apps"),
		readline.PcItem("push"),
		readline.PcItem("run"),
		readline.PcItem("disconnect"),
		readline.PcItem("echo"),
		readline.PcItem("stop"),
		readline.PcItem("exit"),
		readline.PcItem("quit"),
	)
}

func shellHistoryPath() string {
	return filepath.Join(defaultControlSocketDir(), "shell.history")
}
