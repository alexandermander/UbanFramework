package main

import (
	"flag"
	"fmt"
	"os"

	"userve/internal/userve"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "userve failed: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return runServe(os.Args[1:])
	}

	switch os.Args[1] {
	case "serve":
		return runServe(os.Args[2:])
	case "shell":
		return runShell(os.Args[2:])
	case "list", "status", "apps", "disconnect", "run", "stop":
		return runClientCommand(os.Args[1], os.Args[2:])
	case "use", "push", "outputs", "echo", "raw":
		return runClientCommand(os.Args[1], os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return runServe(os.Args[1:])
	}
}

func runServe(args []string) error {
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	tcpPort := fs.Int("tcp-port", userve.DefaultPort, "TCP port to listen on")
	controlSocket := fs.String("control-socket", userve.DefaultControlSocketPath(), "local Unix socket for CLI control")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return userve.RunService(*tcpPort, *controlSocket)
}

func runShell(args []string) error {
	fs := flag.NewFlagSet("shell", flag.ContinueOnError)
	controlSocket := fs.String("control-socket", userve.DefaultControlSocketPath(), "local Unix socket for CLI control")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return userve.RunShell(*controlSocket)
}

func runClientCommand(command string, args []string) error {
	fs := flag.NewFlagSet(command, flag.ContinueOnError)
	controlSocket := fs.String("control-socket", userve.DefaultControlSocketPath(), "local Unix socket for CLI control")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return userve.RunControlCommand(*controlSocket, command, fs.Args())
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  userver serve [--tcp-port PORT] [--control-socket PATH]")
	fmt.Println("  userver shell [--control-socket PATH]")
	fmt.Println("  userver list|status|apps|disconnect|run|stop [--control-socket PATH]")
	fmt.Println("  userver use|push|outputs|echo|raw [--control-socket PATH] <args>")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  userver serve --tcp-port 8080")
	fmt.Println("  userver shell")
	fmt.Println("  userver list")
	fmt.Println("  userver use 2")
	fmt.Println("  userver outputs 50")
}
