package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
)

func runNomadCommand(conn io.ReadWriter, task, allocID string, execcmd ...string) error {
	baseCommands := []string{"alloc", "exec", "-i", "-t=false", fmt.Sprintf("-task=%s", task), allocID}
	cmds := append(baseCommands, execcmd...)

	log.Printf("running command: nomad %s", strings.Join(cmds, " "))
	cmd := exec.Command("nomad", cmds...)

	cmd.Stdin = conn
	cmd.Stdout = conn
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

type NoOpReader struct{}

func (n NoOpReader) Read(p []byte) (int, error) {
	return len(p), nil
}

const DEFAULT_INSTALL_SCRIPT = `command -v socat >/dev/null 2>&1 || {
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update && apt-get install -y socat
  elif command -v apk >/dev/null 2>&1; then
    apk add --no-cache socat
  elif command -v yum >/dev/null 2>&1; then
    yum install -y socat
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y socat
  else
    echo "error: no supported package manager found to install socat" >&2
    exit 1
  fi
}`

func main() {
	task := flag.String("task", "", "task name if alloc contains multiple")
	socatPath := flag.String("socat-path", "/usr/bin/socat", "path to socat binary in task")
	portMap := flag.String("p", "8080:80", "port mapping local_port:<remote_addr(optional)>:remote_port")
	installScript := flag.String("install", DEFAULT_INSTALL_SCRIPT, "install script to run before starting socat")
	allocID := flag.String("alloc-id", "", "alloc id to forward ports for")

	flag.Parse()

	portMapParts := strings.Split(*portMap, ":")

	if len(portMapParts) < 2 {
		log.Fatalf("expected >1 parts (local_port:remote_addr:remote_port) for -p flag, given %d", len(portMapParts))
	}
	if len(*allocID) == 0 {
		log.Fatalf("-alloc-id is required")
	}

	localAddr := "localhost"
	remoteAddr := "localhost"
	localPort := portMapParts[0]
	remotePort := portMapParts[1]

	if len(portMapParts) == 3 {
		remoteAddr = portMapParts[1]
		remotePort = portMapParts[2]
	}

	installCmd := []string{"/bin/sh", "-c", *installScript}

	reader := NoOpReader{}
	writer := os.Stdout

	readWriter := struct {
		io.Reader
		io.Writer
	}{reader, writer}

	err := runNomadCommand(readWriter, *task, *allocID, installCmd...)
	if err != nil {
		log.Printf("nomad exec install command error: %v", err)
		return
	}
	log.Print("Install complete")
	log.Printf("forwarding local port %s to %s:%s", localPort, remoteAddr, remotePort)

	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%s", localAddr, localPort))
	if err != nil {
		log.Fatalf("failed to create local listener: %v", err)
	}
	defer ln.Close()

	log.Printf("started local server: %v", ln.Addr())
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Fatalf("failed to accept new connection: %v", err)
		}

		log.Printf("accepted new connection: %v", conn.RemoteAddr())
		go func(conn net.Conn) {
			defer conn.Close()
			defer log.Printf("closed connection: %v", conn.RemoteAddr())

			soCatCmd := []string{
				*socatPath,
				"-",
				fmt.Sprintf("TCP4:%s:%s", remoteAddr, remotePort),
			}

			err = runNomadCommand(conn, *task, *allocID, soCatCmd...)
			if err != nil {
				log.Printf("nomad exec command error: %v", err)
				return
			}
		}(conn)
	}
}
