package main

import (
	"fmt"
	"log"
	"net"
)

func handleRequest(conn net.Conn) {
	log.Println("Handling request from", conn.RemoteAddr())
	for {
		buf := make([]byte, 1024)
		inLen, err := conn.Read(buf)
		if err != nil {
			log.Println("Error reading", err, conn.RemoteAddr())
			conn.Close()
			return
		}
		_, err = conn.Write(buf[:inLen])
		if err != nil {
			log.Println("Error writing", err, conn.RemoteAddr())
			conn.Close()
			return
		}
	}
}

func main() {
	l, err := net.Listen("tcp", ":1337")
	if err != nil {
		log.Fatal("could not listen", err)
	}
	defer l.Close()
	fmt.Println("Listening.")
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("Error accepting", err)
		}
		go handleRequest(conn)
	}
}
