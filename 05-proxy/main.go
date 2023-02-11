package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"

	"go.arsenm.dev/pcre"
)

// dropCR drops a terminal \r from the data.
func dropCR(data []byte) []byte {
	if len(data) > 0 && data[len(data)-1] == '\r' {
		return data[0 : len(data)-1]
	}
	return data
}

// ScanLines is a split function for a Scanner that returns each line of
// text, stripped of any trailing end-of-line marker. The returned line may
// be empty. The end-of-line marker is one optional carriage return followed
// by one mandatory newline. In regular expression notation, it is `\r?\n`.
// The last non-empty line of input will be returned even if it has no
// newline.
func ScanLines(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if atEOF && len(data) == 0 {
		return 0, nil, nil
	}
	if i := bytes.IndexByte(data, '\n'); i >= 0 {
		// We have a full newline-terminated line.
		return i + 1, dropCR(data[0:i]), nil
	}
	// If we're at EOF, we have a final, non-terminated line. Return it.
	if atEOF {
		return 0, nil, fmt.Errorf("bad")
	}
	// Request more data.
	return 0, nil, nil
}

//const TIMEOUT_SECONDS = 5
//const MAX_REQUESTS = 1_000_000
//const MSG_LEN = 9
//conn.SetReadDeadline(time.Now().Add(TIMEOUT_SECONDS * time.Second))
// log.Println("Handling request from", conn.RemoteAddr())

// var coinRe = pcre.MustCompile(`\b7[A-Za-z0-9]{25,34}\b`)
var coinRe = pcre.MustCompile(`(?<![^ ])7[A-Za-z0-9]{25,34}(?![^ ])`)

//var coinRe = regexp.MustCompile(`\b7[A-Za-z0-9]{25,35}\b`)

func mangleMessage(msg, address string) string {
	return coinRe.ReplaceAllLiteralString(msg, address)
}

const ADDRESS = "7YWHMfk9JZe0LM0g1ZauHuiSxhI"

func proxyOneWay(in, out net.Conn, scanner bufio.Scanner) {
	for scanner.Scan() {
		msg := scanner.Text()
		log.Printf("Got message [%s %s] %s\n", in.RemoteAddr(), out.RemoteAddr(), msg)
		_, err := io.WriteString(out, mangleMessage(msg, ADDRESS)+"\n")
		if err != nil {
			log.Printf("Error writing %s\n", err)
			break
		}
	}
	in.Close()
	out.Close()
}

func handleConnection(conn net.Conn) {
	log.Println("Accepted connection from", conn.RemoteAddr())
	upstream, err := net.Dial("tcp", "chat.protohackers.com:16963")
	if err != nil {
		conn.Close()
		return
	}

	upstreamScanner := bufio.NewScanner(upstream)
	upstreamScanner.Split(ScanLines)
	if !upstreamScanner.Scan() {
		log.Println("Failed to read greeting from upstream")
		conn.Close()
		upstream.Close()
		return
	}

	_, err = io.WriteString(conn, upstreamScanner.Text()+"\n")
	if err != nil {
		log.Println("Failed to write greeting to client")
		conn.Close()
		upstream.Close()
		return
	}

	log.Println("Reading name for", conn.RemoteAddr())
	scanner := bufio.NewScanner(conn)
	scanner.Split(ScanLines)
	if !scanner.Scan() {
		log.Println("Failed to read name")
		conn.Close()
		upstream.Close()
		return
	}
	name := scanner.Text()
	log.Println("Got name", name, "from", conn.RemoteAddr())
	_, err = io.WriteString(upstream, name+"\n")
	if err != nil {
		log.Println("Failed to write name to upstream")
		conn.Close()
		upstream.Close()
		return
	}
	log.Println("kicking off goroutines for ", conn.RemoteAddr())
	go proxyOneWay(upstream, conn, *upstreamScanner)
	go proxyOneWay(conn, upstream, *scanner)
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
		go handleConnection(conn)
	}
}
