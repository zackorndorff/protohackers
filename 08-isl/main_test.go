package main

import (
    "bufio"
    "io"
    "net"
    "testing"
)

type ResponseCase struct {
    Query string
    Response string
}

var responseCases []ResponseCase = []ResponseCase{
    {"1x bar", "1x bar\n"},
    {"1x bar,2x foo", "2x foo\n"},
    {"2x bar,1x foo", "2x bar\n"},
}

func TestOneResponse(t *testing.T) {
    for _, c := range responseCases {
        result, err := getResponse(c.Query)
        if err != nil {
            t.Errorf("Failed to get response (got error %s) [case %+v]", err, c)
        }
        if result != c.Response {
            t.Errorf("Bad response (not as expected). Got %s. Expected %s.", result, c.Response)
        }
    }
}

func TestConn(t *testing.T) {
	server, client := net.Pipe()
	go func() {
		defer server.Close()
		handleConn(server)
	}()
	defer client.Close()
        bufc := bufio.NewWriter(client)

	io.WriteString(bufc, "2x calling birds,3x French hens\n")
        bufc.Flush()
        sc := bufio.NewScanner(client)
        if !sc.Scan() {
            t.Fatalf("failed to scan: %s", sc.Err())
        }
        if sc.Text() != "3x French hens" {
            t.Fatalf("got unexpected result %s", sc.Text())
        }
}

func TestDoubleConn(t *testing.T) {
	server, client := net.Pipe()
	go func() {
		defer server.Close()
		handleConn(server)
	}()
        bufc := bufio.NewWriter(client)
	defer client.Close()

	io.WriteString(bufc, "2x calling birds,3x French hens\n")
	io.WriteString(bufc, "3x French hens,5x golden rings\n")
        bufc.Flush()
        sc := bufio.NewScanner(client)
        if !sc.Scan() {
            t.Fatalf("failed to scan(1): %s", sc.Err())
        }
        if sc.Text() != "3x French hens" {
            t.Fatalf("got unexpected result(1) %s", sc.Text())
        }
        if !sc.Scan() {
            t.Fatalf("failed to scan(2): %s", sc.Err())
        }
        if sc.Text() != "5x golden rings" {
            t.Fatalf("got unexpected result(2) %s", sc.Text())
        }
}
