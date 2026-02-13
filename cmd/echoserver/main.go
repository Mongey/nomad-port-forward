package main

import (
	"fmt"
	"io"
	"net"
	"os"
)

func main() {
	ln, err := net.Listen("tcp", ":7777")
	if err != nil {
		fmt.Fprintf(os.Stderr, "listen: %v\n", err)
		os.Exit(1)
	}
	defer ln.Close()
	fmt.Println("LISTENING")

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		go func(c net.Conn) {
			defer c.Close()
			io.Copy(c, c)
		}(conn)
	}
}
