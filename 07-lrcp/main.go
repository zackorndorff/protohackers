package main

import (
	"bufio"
	"fmt"
	"log"
	"net"

	"z10f.com/golang/protohackers/07/lrcp"
)

func reverse(s []byte) []byte {
	rev := s[:]
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev
}

func handleRequest(conn net.Conn) {
	log.Println("Handling request from", conn.RemoteAddr())
	defer conn.Close()
	for s := bufio.NewScanner(conn); s.Scan(); {
		data := s.Bytes()
		log.Println("APPLICATION: Got input", string(data))
		data = reverse(data)
		data = append(data, '\n')
		log.Println("APPLICATION: Writing reply", string(data))
		_, err := conn.Write(data)
		if err != nil {
			log.Println("error writing", err)
			return
		}
	}
}

func main() {
	l, err := lrcp.Listen("lrcp", ":1337")
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
