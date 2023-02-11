package main

import (
	"log"
	"net"
	"strings"
)

var crap map[string]string

func handleRequest(req []byte, source net.Addr) ([]byte, error) {
	sreq := string(req)
	before, after, had_equals := strings.Cut(sreq, "=")
	if had_equals {
		log.Printf("[%s]\tInsert: %s=%s\n", source, before, after)
		// insert
		if before == "version" {
		} else {
			crap[before] = after
		}
		return []byte{}, nil
	} else {
		// query
		if before == "version" {
			log.Printf("[%s]\tVersion query\n", source)
			return []byte("version=zudp-1.0"), nil
		} else {
			log.Printf("[%s]\tQuery for %s\n", source, before)
			res := append([]byte(before), "="...)
			if val, ok := crap[before]; ok {
				res = append(res, []byte(val)...)
			}
			log.Printf("[%s]\tQuery returned %s\n", source, res)
			return res, nil
		}
	}
}

func resetForTesting() {
	crap = make(map[string]string)
}

func main() {
	crap = make(map[string]string)

	conn, err := net.ListenPacket("udp", ":1337")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	for {
		buf := make([]byte, 1024)
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			continue
		}
		if n >= 1000 {
			continue
		}
		log.Printf("[%s] len(buf) was %d, n was %d", addr, len(buf), n)
		result, err := handleRequest(buf[:n], addr)
		if err != nil {
			log.Printf("Got error serving %s: %s\n", addr, err)
			continue
		}
		if len(result) > 0 {
			conn.WriteTo(result, addr)
		}
	}
}
