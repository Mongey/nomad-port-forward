package main

import (
	"bytes"
	_ "embed"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
)

//go:embed embed/tcpfwd-linux-amd64
var tcpfwdAmd64 []byte

//go:embed embed/tcpfwd-linux-arm64
var tcpfwdArm64 []byte

func runNomadCommand(stdin io.Reader, stdout io.Writer, task, allocID string, execcmd ...string) error {
	baseCommands := []string{"alloc", "exec", "-i", "-t=false", fmt.Sprintf("-task=%s", task), allocID}
	cmds := append(baseCommands, execcmd...)

	log.Printf("running command: nomad %s", strings.Join(cmds, " "))
	cmd := exec.Command("nomad", cmds...)

	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

func mapArch(uname string) (string, error) {
	arch := strings.TrimSpace(uname)
	switch arch {
	case "x86_64":
		return "amd64", nil
	case "aarch64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported architecture: %s", arch)
	}
}

func detectArch(task, allocID string) (string, error) {
	var buf bytes.Buffer
	err := runNomadCommand(nil, &buf, task, allocID, "uname", "-m")
	if err != nil {
		return "", fmt.Errorf("failed to detect arch: %w", err)
	}

	return mapArch(buf.String())
}

func uploadBinary(task, allocID, arch string) error {
	var bin []byte
	switch arch {
	case "amd64":
		bin = tcpfwdAmd64
	case "arm64":
		bin = tcpfwdArm64
	default:
		return fmt.Errorf("unsupported architecture: %s", arch)
	}

	return runNomadCommand(
		bytes.NewReader(bin),
		os.Stdout,
		task, allocID,
		"/bin/sh", "-c", "cat > /tmp/tcpfwd && chmod +x /tmp/tcpfwd",
	)
}

func parsePortMap(s string) (localPort, remoteAddr, remotePort string, err error) {
	parts := strings.Split(s, ":")
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("expected >1 parts (local_port:remote_addr:remote_port) for -p flag, given %d", len(parts))
	}

	localPort = parts[0]
	remoteAddr = "localhost"
	remotePort = parts[1]

	if len(parts) == 3 {
		remoteAddr = parts[1]
		remotePort = parts[2]
	}
	return localPort, remoteAddr, remotePort, nil
}

func main() {
	task := flag.String("task", "", "task name if alloc contains multiple")
	portMap := flag.String("p", "8080:80", "port mapping local_port:<remote_addr(optional)>:remote_port")
	allocID := flag.String("alloc-id", "", "alloc id to forward ports for")

	flag.Parse()

	localPort, remoteAddr, remotePort, err := parsePortMap(*portMap)
	if err != nil {
		log.Fatalf("%v", err)
	}
	if len(*allocID) == 0 {
		log.Fatalf("-alloc-id is required")
	}

	arch, err := detectArch(*task, *allocID)
	if err != nil {
		log.Fatalf("failed to detect architecture: %v", err)
	}
	log.Printf("detected architecture: %s", arch)

	err = uploadBinary(*task, *allocID, arch)
	if err != nil {
		log.Fatalf("failed to upload tcpfwd binary: %v", err)
	}
	log.Print("tcpfwd binary uploaded")

	log.Printf("forwarding local port %s to %s:%s", localPort, remoteAddr, remotePort)

	ln, err := net.Listen("tcp", fmt.Sprintf("localhost:%s", localPort))
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

			tcpfwdCmd := []string{
				"/tmp/tcpfwd",
				fmt.Sprintf("%s:%s", remoteAddr, remotePort),
			}

			err = runNomadCommand(conn, conn, *task, *allocID, tcpfwdCmd...)
			if err != nil {
				log.Printf("nomad exec command error: %v", err)
				return
			}
		}(conn)
	}
}
