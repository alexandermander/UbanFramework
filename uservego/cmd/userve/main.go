package main

import (
	"flag"
	"fmt"
	"os"

	"userve/internal/userve"
)

func main() {
	tcpPort := flag.Int("tcp-port", userve.DefaultPort, "TCP port to listen on")
	flag.Parse()

	if err := userve.Run(*tcpPort); err != nil {
		fmt.Fprintf(os.Stderr, "userve failed: %v\n", err)
		os.Exit(1)
	}
}
