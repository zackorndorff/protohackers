package lrcp

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"testing"
)

type ParseCase struct {
	PacketData    string
	Parsed        interface{}
	ShouldError   bool
	ExpectedError error
}

var parseCases = []ParseCase{
	{"", nil, true, ErrMalformedPacket},
	{"/lol/notapackettype/", nil, true, ErrInvalidPacketType},
	{"connect/1/", nil, true, ErrMalformedPacket},
	{"/connect/", nil, true, ErrMalformedPacket},
	{"/connect/1", nil, true, ErrMalformedPacket},
	{"/connect/1/foo/", nil, true, ErrMalformedPacket},
	{"/connect/1\\/", nil, true, nil},
	{"/connect/notanid/", nil, true, ErrInvalidSessionID},
	{"/connect/1/", ConnectPacket{SessionID: 1}, false, nil},
	{"/connect/1234567/", ConnectPacket{SessionID: 1234567}, false, nil},
	{"/data/0/hello/", nil, true, ErrMalformedPacket},
	{"/data/notanid/0/hello/", nil, true, ErrInvalidSessionID},
	{"/data/1234567/badpos/hello/", nil, true, nil},
	{"/data/1234567/0/hello/", DataPacket{SessionID: 1234567, Position: 0, Data: []byte("hello")}, false, nil},
	{"/data/1234567/0/h\\\\el\\/lo/", DataPacket{SessionID: 1234567, Position: 0, Data: []byte("h\\el/lo")}, false, nil},
	{"/ack/1234567/5/", AckPacket{SessionID: 1234567, Length: 5}, false, nil},
	{"/ack/badid/5/", nil, true, ErrInvalidSessionID},
	{"/ack/1234567/notalen/", nil, true, nil},
	{"/ack/1234567/5/extrastuff/", nil, true, ErrMalformedPacket},
	{"/close/1234567/", ClosePacket{SessionID: 1234567}, false, nil},
	{"/close/badid/", nil, true, ErrInvalidSessionID},
	{"/close/1234567/extra/", nil, true, ErrMalformedPacket},
}

func TestParsePacket(t *testing.T) {
	for idx, i := range parseCases {
		caseBytes := []byte(i.PacketData)
		parsed, err := parsePacket(caseBytes)
		didError := (err != nil)
		if i.ShouldError != didError {
			t.Errorf("case %d failed: should error was %t but did error was %t",
				idx, i.ShouldError, didError)
		} else if i.ExpectedError != nil && !errors.Is(err, i.ExpectedError) {
			t.Errorf("case %d failed: expected error was %s but error was %s",
				idx, i.ExpectedError, err)
		} else if err != nil && !reflect.DeepEqual(i.Parsed, parsed) {
			t.Errorf("case %d failed; expected %#v, got %#v", idx, i.Parsed, parsed)
		}
	}
}

func TestSerializePacket(t *testing.T) {
	for idx, i := range parseCases {
		if i.ExpectedError != nil || i.ShouldError {
			continue
		}
		serialized := serializePacket(i.Parsed)
		expected := []byte(i.PacketData)
		if !bytes.Equal(serialized, expected) {
			t.Errorf("case %d failed: expected %v, got %v", idx, string(expected), string(serialized))
		}
	}
}

// I didn't know what would happen here, so I tested it
func TestBufferBehavior(t *testing.T) {
	var buf bytes.Buffer
	buf.Write([]byte("foo"))
	fmt.Println(buf.Bytes())
	if buf.Len() != 3 {
		t.Fatal("this won't work")
	}
}
