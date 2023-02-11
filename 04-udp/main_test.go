package main

import (
	"bytes"
	"net"
	"testing"
)

func TestSmokeTest(t *testing.T) {
	resetForTesting()
	source := net.IPAddr{
		IP: net.ParseIP("0.0.0.0"),
	}
	res, err := handleRequest([]byte("foo"), &source)
	if err != nil {
		t.Error("should not have failed to query foo")
	}
	if !bytes.Equal(res, []byte("foo=")) {
		t.Error("should not have been a value for foo")
	}
}

func TestStoreItem(t *testing.T) {
	resetForTesting()
	source := net.IPAddr{
		IP: net.ParseIP("0.0.0.0"),
	}
	res, err := handleRequest([]byte("foo=bar"), &source)
	if err != nil {
		t.Fatal("should not have failed to store foo")
	}
	if len(res) > 0 {
		t.Error("should not have gotten reply to store")
	}

	res, err = handleRequest([]byte("foo"), &source)
	if err != nil {
		t.Error("should not have failed to query foo")
	}
	if !bytes.Equal(res, []byte("foo=bar")) {
		t.Error("should have been a value for foo")
	}
}

func TestStoreFancyItem(t *testing.T) {
	resetForTesting()
	source := net.IPAddr{
		IP: net.ParseIP("0.0.0.0"),
	}
	res, err := handleRequest([]byte("foo=bar=1"), &source)
	if err != nil {
		t.Fatal("should not have failed to store foo")
	}
	if len(res) > 0 {
		t.Error("should not have gotten reply to store")
	}

	res, err = handleRequest([]byte("foo"), &source)
	if err != nil {
		t.Error("should not have failed to query foo")
	}
	if !bytes.Equal(res, []byte("foo=bar=1")) {
		t.Error("should have been a value for foo")
	}
}
