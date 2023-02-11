package main

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestEmpty(t *testing.T) {
	c := Connection{}
	if c.Query(time.UnixMilli(0), time.UnixMilli(100)) != 0 {
		t.Error("Failed to return 0 for empty result")
	}
}

func TestOne(t *testing.T) {
	c := Connection{}
	c.Insert(Price{Timestamp: time.UnixMilli(123), Price: 4})
	if c.Query(time.UnixMilli(0), time.UnixMilli(200)) != 4 {
		t.Error("Could not query obvious price")
	}
}

func TestTwo(t *testing.T) {
	c := Connection{}
	c.Insert(Price{Timestamp: time.UnixMilli(123), Price: 4})
	c.Insert(Price{Timestamp: time.UnixMilli(130), Price: 6})
	if c.Query(time.UnixMilli(0), time.UnixMilli(200)) != 5 {
		t.Error("Could not query obvious average")
	}
}

func TestConn(t *testing.T) {
	server, client := net.Pipe()
	go func() {
		defer server.Close()
		handleRequest(server)
	}()
	client.Write([]byte{'I'})
	ins := InsertRequest{
		Timestamp: 123,
		Price:     100,
	}
	binary.Write(client, binary.BigEndian, &ins)
	client.Write([]byte{'Q'})
	q := QueryRequest{
		Mintime: 0,
		Maxtime: 200,
	}
	binary.Write(client, binary.BigEndian, &q)
	var result int32
	err := binary.Read(client, binary.BigEndian, &result)
	if err != nil {
		t.Fatal(err)
	}
	if result != 100 {
		t.Fatalf("Read incorrect result back: %d.", result)
	}
	client.Close()
}

func TestTimeout(t *testing.T) {
	server, client := net.Pipe()
	go func() {
		defer server.Close()
		handleRequest(server)
	}()
	time.Sleep((TIMEOUT_SECONDS + 1) * time.Second)
	buf := make([]byte, 1)
	client.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, err := client.Read(buf)
	if err != io.EOF {
		t.Fatalf("Failed to time out: %s", err)
	}
}
