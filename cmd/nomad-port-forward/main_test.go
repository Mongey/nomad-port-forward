package main

import (
	"bytes"
	"context"
	"io"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestParsePortMap(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		localPort  string
		remoteAddr string
		remotePort string
		wantErr    bool
	}{
		{"two parts", "8080:80", "8080", "localhost", "80", false},
		{"three parts", "8080:10.0.0.1:80", "8080", "10.0.0.1", "80", false},
		{"one part", "8080", "", "", "", true},
		{"empty", "", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lp, ra, rp, err := parsePortMap(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if lp != tt.localPort {
				t.Errorf("localPort = %q, want %q", lp, tt.localPort)
			}
			if ra != tt.remoteAddr {
				t.Errorf("remoteAddr = %q, want %q", ra, tt.remoteAddr)
			}
			if rp != tt.remotePort {
				t.Errorf("remotePort = %q, want %q", rp, tt.remotePort)
			}
		})
	}
}

func TestMapArch(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{"x86_64", "x86_64", "amd64", false},
		{"x86_64 trailing newline", "x86_64\n", "amd64", false},
		{"aarch64", "aarch64", "arm64", false},
		{"armv7l", "armv7l", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := mapArch(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("mapArch(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func buildBinary(t *testing.T, pkg string) string {
	t.Helper()
	bin := t.TempDir() + "/" + pkg
	arch := runtime.GOARCH
	build := exec.Command("go", "build", "-o", bin, "../../cmd/"+pkg)
	build.Env = append(build.Environ(), "GOOS=linux", "GOARCH="+arch, "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("failed to build %s: %v\n%s", pkg, err, out)
	}
	return bin
}

func TestTcpfwdInContainer(t *testing.T) {
	tcpfwdBin := buildBinary(t, "tcpfwd")
	echoserverBin := buildBinary(t, "echoserver")

	tests := []struct {
		name  string
		image string
	}{
		{"debian", "debian:bookworm-slim"},
		{"alpine", "alpine:3.20"},
		{"fedora", "fedora:41"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()

			c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
				ContainerRequest: testcontainers.ContainerRequest{
					Image:      tt.image,
					Cmd:        []string{"/bin/sh", "-c", "sleep 300"},
					WaitingFor: wait.ForExec([]string{"/bin/sh", "-c", "true"}),
				},
				Started: true,
			})
			testcontainers.CleanupContainer(t, c)
			if err != nil {
				t.Fatalf("failed to start container: %v", err)
			}

			// Copy binaries into the container
			err = c.CopyFileToContainer(ctx, tcpfwdBin, "/tmp/tcpfwd", 0o755)
			if err != nil {
				t.Fatalf("failed to copy tcpfwd to container: %v", err)
			}
			err = c.CopyFileToContainer(ctx, echoserverBin, "/tmp/echoserver", 0o755)
			if err != nil {
				t.Fatalf("failed to copy echoserver to container: %v", err)
			}

			// Verify tcpfwd exits 1 with no args
			exitCode, output, err := c.Exec(ctx, []string{"/tmp/tcpfwd"})
			if err != nil {
				t.Fatalf("failed to exec tcpfwd: %v", err)
			}
			if exitCode != 1 {
				buf := new(strings.Builder)
				io.Copy(buf, output)
				t.Fatalf("expected exit code 1 (no args), got %d: %s", exitCode, buf.String())
			}

			// Start echo server in background
			_, _, err = c.Exec(ctx, []string{"/bin/sh", "-c", "nohup /tmp/echoserver &"})
			if err != nil {
				t.Fatalf("failed to start echo server: %v", err)
			}

			// Wait for echo server to be ready by polling the port
			exitCode = 1
			for i := 0; i < 20; i++ {
				exitCode, _, _ = c.Exec(ctx, []string{"/bin/sh", "-c", "cat < /dev/null > /dev/tcp/localhost/7777 2>/dev/null || (echo | /tmp/tcpfwd localhost:7777 2>/dev/null && true) || false"})
				if exitCode == 0 {
					break
				}
				c.Exec(ctx, []string{"sleep", "0.1"})
			}

			// Run tcpfwd to connect to the echo server
			exitCode, reader, err := c.Exec(ctx, []string{
				"/bin/sh", "-c",
				"echo PING | timeout 2 /tmp/tcpfwd localhost:7777 2>/dev/null",
			})
			if err != nil {
				t.Fatalf("failed to run tcpfwd: %v", err)
			}

			var buf bytes.Buffer
			io.Copy(&buf, reader)
			got := buf.String()

			if !strings.Contains(got, "PING") {
				t.Fatalf("expected output to contain PING, got (exit=%d): %q", exitCode, got)
			}
		})
	}
}
