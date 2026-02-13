package main

import (
	"io"
	"net"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		os.Exit(1)
	}
	conn, err := net.Dial("tcp", os.Args[1])
	if err != nil {
		os.Exit(1)
	}
	defer conn.Close()

	go func() {
		io.Copy(conn, os.Stdin)
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.CloseWrite()
		}
	}()
	io.Copy(os.Stdout, conn)
}
