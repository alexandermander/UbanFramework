package control

import (
	"fmt"
	"strconv"
	"strings"

	"userve/internal/hub"
	"userve/internal/stream"
)

type commandService interface {
	SendRaw(text string) (bool, string)
	Echo(text string) (bool, string)
	GetApps() (bool, string)
	Disconnect() (bool, string)
	Push(path string) (bool, string)
	ExecuteUploaded() (bool, string)
}

type Service struct {
	hub      *hub.Hub
	commands commandService
	tail     *stream.TailSink
	shutdown func()
}

func NewService(h *hub.Hub, commands commandService, tail *stream.TailSink, shutdown func()) *Service {
	return &Service{
		hub:      h,
		commands: commands,
		tail:     tail,
		shutdown: shutdown,
	}
}

func (s *Service) HandleControlRequest(req ControlRequest) ControlResponse {
	command := strings.TrimSpace(req.Command)
	switch command {
	case "list":
		return ControlResponse{OK: true, Lines: s.listLines()}
	case "status":
		return ControlResponse{OK: true, Lines: s.statusLines()}
	case "use":
		if len(req.Args) != 1 {
			return ControlResponse{OK: false, Lines: []string{"ERR use requires exactly one connection id"}}
		}
		id, err := strconv.Atoi(req.Args[0])
		if err != nil {
			return ControlResponse{OK: false, Lines: []string{fmt.Sprintf("ERR invalid connection id: %s", req.Args[0])}}
		}
		if !s.hub.SetActive(id) {
			return ControlResponse{OK: false, Lines: []string{fmt.Sprintf("ERR unknown connection id %d", id)}}
		}
		return ControlResponse{OK: true, Lines: []string{fmt.Sprintf("Active connection set to #%d", id)}}
	case "outputs":
		limit := 20
		if len(req.Args) > 1 {
			return ControlResponse{OK: false, Lines: []string{"ERR outputs accepts at most one limit"}}
		}
		if len(req.Args) == 1 {
			value, err := strconv.Atoi(req.Args[0])
			if err != nil || value < 0 {
				return ControlResponse{OK: false, Lines: []string{fmt.Sprintf("ERR invalid output limit: %s", req.Args[0])}}
			}
			limit = value
		}
		lines := s.tail.Lines(limit)
		if len(lines) == 0 {
			return ControlResponse{OK: true, Lines: []string{"No buffered output."}}
		}
		return ControlResponse{OK: true, Lines: lines}
	case "apps":
		return responseFromResult(s.commands.GetApps())
	case "disconnect":
		return responseFromResult(s.commands.Disconnect())
	case "push":
		if len(req.Args) != 1 {
			return ControlResponse{OK: false, Lines: []string{"ERR push requires a file path"}}
		}
		return responseFromResult(s.commands.Push(req.Args[0]))
	case "run":
		return responseFromResult(s.commands.ExecuteUploaded())
	case "echo":
		return responseFromResult(s.commands.Echo(strings.Join(req.Args, " ")))
	case "raw":
		return responseFromResult(s.commands.SendRaw(strings.Join(req.Args, " ")))
	case "stop":
		if s.shutdown != nil {
			s.shutdown()
		}
		return ControlResponse{OK: true, Lines: []string{"Service shutting down."}}
	default:
		return ControlResponse{OK: false, Lines: []string{fmt.Sprintf("ERR unknown control command %q", command)}}
	}
}

func (s *Service) statusLines() []string {
	snap, ok := s.hub.ActiveSnapshot()
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

func (s *Service) listLines() []string {
	snaps := s.hub.Snapshots()
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

func responseFromResult(ok bool, line string) ControlResponse {
	return ControlResponse{OK: ok, Lines: []string{line}}
}
