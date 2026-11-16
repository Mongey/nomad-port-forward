# nomad-port-forward

## Description

`nomad-port-forward` is a simple utility to forward ports from a Nomad job to a local machine.

Thanks to @picatz for showing how to do this in https://github.com/hashicorp/nomad/issues/6925

## Install

### Homebrew

```bash
brew install Mongey/tap/nomad-port-forward
```

### From source

Requires Go 1.27.4 or newer. Generate the embedded Linux proxies before building.

```bash
git clone https://github.com/Mongey/nomad-port-forward.git
cd nomad-port-forward
go generate ./...
go install ./cmd/nomad-port-forward
```

### Download binary

Download a pre-built binary from the [releases page](https://github.com/Mongey/nomad-port-forward/releases).

## Usage

```bash
nomad-port-forward -alloc-id <alloc-id> -task nginx -p 8080:localhost:80
```

## How it works

`nomad-port-forward` embeds a tiny static Go TCP proxy binary (`tcpfwd`) for linux/amd64 and linux/arm64. At runtime it:

1. Detects the target allocation's architecture via `nomad alloc exec ... uname -m`
2. Uploads the matching `tcpfwd` binary into the allocation at `/tmp/tcpfwd`
3. For each local connection, runs `nomad alloc exec ... /tmp/tcpfwd addr:port` to proxy traffic

No package manager, network access, or root permissions are needed inside the allocation.

## Development

With Go installed and Docker running, generate the embedded proxies and run the checks:

```bash
go generate ./...
go test -race -count=1 -timeout=5m ./...
go vet ./...
```

The tests check port parsing, architecture selection, TCP forwarding, a 1 MiB payload, and the proxy in Debian, Alpine, and Fedora containers.

Use GoReleaser 2.20.2 to build release archives locally without publishing:

```bash
goreleaser release --snapshot --clean
```

The release configuration retains `brews` to preserve the existing Homebrew formula. GoReleaser reports a deprecation warning until the tap is migrated to a cask.
