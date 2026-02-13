package main

import (
	"bytes"
	"crypto/rand"
	"io"
	"net"
	"os/exec"
	"testing"
)

func buildTcpfwd(t *testing.T) string {
	t.Helper()
	bin := t.TempDir() + "/tcpfwd"
	build := exec.Command("go", "build", "-o", bin, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build tcpfwd: %v\n%s", err, out)
	}
	return bin
}

func TestTcpfwdProxiesTraffic(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start echo server: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	bin := buildTcpfwd(t)

	cmd := exec.Command(bin, ln.Addr().String())
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("failed to get stdin pipe: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed to get stdout pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start tcpfwd: %v", err)
	}
	defer cmd.Process.Kill()

	msg := "hello tcpfwd\n"
	if _, err := io.WriteString(stdin, msg); err != nil {
		t.Fatalf("failed to write to stdin: %v", err)
	}
	stdin.Close()

	got, err := io.ReadAll(stdout)
	if err != nil {
		t.Fatalf("failed to read stdout: %v", err)
	}

	if string(got) != msg {
		t.Fatalf("expected %q, got %q", msg, string(got))
	}

	cmd.Wait()
}

func TestTcpfwdExitCode1WithNoArgs(t *testing.T) {
	bin := buildTcpfwd(t)

	cmd := exec.Command(bin)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code, got nil error")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Fatalf("expected exit code 1, got %d", exitErr.ExitCode())
		}
	} else {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
}

func TestTcpfwdExitCode1BadAddress(t *testing.T) {
	bin := buildTcpfwd(t)

	cmd := exec.Command(bin, "127.0.0.1:1")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit code, got nil error")
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() != 1 {
			t.Fatalf("expected exit code 1, got %d", exitErr.ExitCode())
		}
	} else {
		t.Fatalf("expected *exec.ExitError, got %T: %v", err, err)
	}
}

func TestTcpfwdLargePayload(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to start echo server: %v", err)
	}
	defer ln.Close()

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				io.Copy(c, c)
			}(conn)
		}
	}()

	bin := buildTcpfwd(t)

	// Generate 1MB of random data
	payload := make([]byte, 1<<20)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("failed to generate random data: %v", err)
	}

	cmd := exec.Command(bin, ln.Addr().String())
	cmd.Stdin = bytes.NewReader(payload)

	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("tcpfwd failed: %v", err)
	}

	if !bytes.Equal(out, payload) {
		t.Fatalf("payload mismatch: sent %d bytes, got %d bytes", len(payload), len(out))
	}
}
