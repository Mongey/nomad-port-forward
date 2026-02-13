package main

//go:generate env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o embed/tcpfwd-linux-amd64 ../../cmd/tcpfwd
//go:generate env GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o embed/tcpfwd-linux-arm64 ../../cmd/tcpfwd
