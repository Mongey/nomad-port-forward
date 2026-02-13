package main

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func runInstallScript(t *testing.T, image string) string {
	t.Helper()
	ctx := context.Background()

	script := DEFAULT_INSTALL_SCRIPT + " && socat -V"
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      image,
			Cmd:        []string{"/bin/sh", "-c", script},
			WaitingFor: wait.ForExit(),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, c)
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	logs, err := c.Logs(ctx)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	buf := new(strings.Builder)
	io.Copy(buf, logs)
	return buf.String()
}

func TestInstallScriptInstallsSocat(t *testing.T) {
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
			output := runInstallScript(t, tt.image)
			if !strings.Contains(output, "socat version") {
				t.Fatalf("socat not installed correctly, output:\n%s", output)
			}
		})
	}
}

func TestInstallScriptSkipsIfSocatPresent(t *testing.T) {
	ctx := context.Background()

	// Pre-install socat, then run the install script.
	// The script should short-circuit at `command -v socat` and not run apt-get again.
	script := `apt-get update >/dev/null 2>&1 && apt-get install -y socat >/dev/null 2>&1 && ` +
		DEFAULT_INSTALL_SCRIPT + ` && echo "INSTALL_SCRIPT_DONE"`
	c, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:      "debian:bookworm-slim",
			Cmd:        []string{"/bin/sh", "-c", script},
			WaitingFor: wait.ForExit(),
		},
		Started: true,
	})
	testcontainers.CleanupContainer(t, c)
	if err != nil {
		t.Fatalf("failed to start container: %v", err)
	}

	logs, err := c.Logs(ctx)
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	buf := new(strings.Builder)
	io.Copy(buf, logs)
	output := buf.String()

	if !strings.Contains(output, "INSTALL_SCRIPT_DONE") {
		t.Fatalf("install script failed, output:\n%s", output)
	}
}
