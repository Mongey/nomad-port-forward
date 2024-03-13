# nomad-port-forward

## Description

`nomad-port-forward` is a simple utility to forward ports from a Nomad job to a local machine.

## Usage

```bash
nomad-port-forward -alloc-id <alloc-id> -task nginx -p 8080:localhost:80
```

## How it works

`nomad-port-forward` installs socat inside the target nomad allocation and forwards the local port to the remote port.
