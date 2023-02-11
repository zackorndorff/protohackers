package main

import (
	"bytes"
	"net"
	"testing"
)

type ReverseTest struct {
	String  string
	Reverse string
}

var reverses = []ReverseTest{
	{"hello", "olleh"},
	{"four", "ruof"},
}

func TestReverse(t *testing.T) {
	for idx, i := range reverses {
		rev := reverse([]byte(i.String))
		if !bytes.Equal(rev, []byte(i.Reverse)) {
			t.Errorf("case %d failed: expected %#v, got %#v", idx, i.Reverse, rev)
		}
	}
}

func readToNewline(conn net.Conn) ([]byte, error) {
	buf := []byte{}
	for len(buf) == 0 || buf[len(buf)-1] != '\n' {
		tempBuf := make([]byte, 1)
		_, err := conn.Read(tempBuf)
		if err != nil {
			return nil, err
		}
		buf = append(buf, tempBuf[0])
	}
	return buf, nil
}

func TestConn(t *testing.T) {
	server, client := net.Pipe()
	go func() {
		defer server.Close()
		handleRequest(server)
	}()
	_, err := client.Write([]byte("hello\n"))
	if err != nil {
		t.Fatal(err)
	}
	buf, err := readToNewline(client)
	if err != nil {
		t.Fatal(err)
	}
	expected := "olleh\n"
	if string(buf) != expected {
		t.Fatalf("expected %#v, got %#v", expected, string(buf))
	}
	_, err = client.Write([]byte("\\also / hello \n"))
	if err != nil {
		t.Fatal(err)
	}
	buf, err = readToNewline(client)
	if err != nil {
		t.Fatal(err)
	}
	expected = " olleh / osla\\\n"
	if string(buf) != expected {
		t.Fatalf("expected %#v, got %#v", expected, string(buf))
	}
	client.Close()
}
