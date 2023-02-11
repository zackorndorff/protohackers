package main

import (
	//"encoding/binary"

	"bufio"
	"io"
	"log"
	"net"
	"strings"
	"testing"
)

func TestConnectMsg(t *testing.T) {
	channel := newChannel()
	go channel.Handle()

	server1, client1 := net.Pipe()
	defer client1.Close()
	go handleConnection(server1, channel)
	sc := bufio.NewScanner(client1)
	if !sc.Scan() {
		t.Fatal("could not scan")
	}
	log.Println(sc.Text())
	io.WriteString(client1, "client1\n")
	if !sc.Scan() {
		t.Fatal("could not scan")
	}
	log.Println(sc.Text())

	server2, client2 := net.Pipe()
	go handleConnection(server2, channel)
	sc2 := bufio.NewScanner(client2)
	if !sc2.Scan() {
		t.Fatal("could not scan")
	}
	log.Println(sc2.Text())
	io.WriteString(client2, "client2\n")

	if !sc2.Scan() {
		t.Fatal("could not scan")
	}
	data := sc2.Text()
	if !(strings.Contains(data, "contains:") &&
		strings.Contains(data, "client1")) {
		t.Errorf("contains string not correctly sent to client1")
	}
	log.Println("resp1", data)

	io.WriteString(client1, "hello world\n")

	if !sc2.Scan() {
		t.Fatal("could not scan")
	}
	data = sc2.Text()
	log.Println("resp2", data)
	if !strings.Contains(data, "hello world") {
		t.Errorf("client1 message not sent to client2")
	}

	client2.Close()

	if !sc.Scan() {
		t.Fatal("could not scan")
	}
	data = sc.Text()
	if !strings.Contains(data, "client2") {
		t.Errorf("entry message not properly sent to client1")
	}

	if !sc.Scan() {
		t.Fatal("could not scan")
	}
	data = sc.Text()
	if !strings.Contains(data, "client2") {
		t.Errorf("left message not properly sent to client1")
	}
}

//func TestTimeout(t *testing.T) {
//	server, client := net.Pipe()
//	go func() {
//		defer server.Close()
//		handleRequest(server)
//	}()
//	time.Sleep((TIMEOUT_SECONDS + 1) * time.Second)
//	buf := make([]byte, 1)
//	client.SetReadDeadline(time.Now().Add(1 * time.Second))
//	_, err := client.Read(buf)
//	if err != io.EOF {
//		t.Fatalf("Failed to time out: %s", err)
//	}
//}
