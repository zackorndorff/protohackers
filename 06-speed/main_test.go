package main

import (
	"bytes"
	"encoding/hex"
	"io"
	"log"
	"reflect"
	"strings"
	"testing"

	"z10f.com/golang/protohackers/06/core"
)

func mustDecodeHex(hexdata string) []byte {
	hexdata = strings.ReplaceAll(hexdata, " ", "")
	data, err := hex.DecodeString(hexdata)
	if err != nil {
		panic(err)
	}
	return data
}

type UnmarshalCase struct {
	Data          []byte
	ExpectedValue interface{}
}

var unmarshalCases []UnmarshalCase = []UnmarshalCase{
	{mustDecodeHex("20 04 55 4e 31 58 00 00 03 e8"), &MsgPlate{Plate: "UN1X", Timestamp: 1000}},
	{mustDecodeHex("40 00 00 00 0a"), &MsgWantHeartbeat{Interval: 10}},
	{mustDecodeHex("40 00 00 04 db"), &MsgWantHeartbeat{Interval: 1243}},
	{mustDecodeHex("80 00 42 00 64 00 3c"), &MsgIAmCamera{Road: 66, Mile: 100, Limit: 60}},
	{mustDecodeHex("80 01 70 04 d2 00 28"), &MsgIAmCamera{Road: 368, Mile: 1234, Limit: 40}},
	{mustDecodeHex("81 01 00 42"), &MsgIAmDispatcher{NumRoads: 1, Roads: []uint16{66}}},
	{mustDecodeHex("81 03 00 42 01 70 13 88"), &MsgIAmDispatcher{NumRoads: 3, Roads: []uint16{66, 368, 5000}}},
}

var marshalCases []UnmarshalCase = []UnmarshalCase{
	{mustDecodeHex("10 03 62 61 64"), &MsgError{Msg: "bad"}},
	{mustDecodeHex("10 0b 69 6c 6c 65 67 61 6c 20 6d 73 67"), &MsgError{Msg: "illegal msg"}},
	{mustDecodeHex("21 04 55 4e 31 58 00 42 00 64 00 01 e2 40 00 6e 00 01 e3 a8 27 10"), &core.Ticket{
		Plate:      "UN1X",
		Road:       66,
		Mile1:      100,
		Timestamp1: 123456,
		Mile2:      110,
		Timestamp2: 123816,
		Speed:      10000,
	}},
	{mustDecodeHex("21 07 52 45 30 35 42 4b 47 01 70 04 d2 00 0f 42 40 04 d3 00 0f 42 7c 17 70"), &core.Ticket{
		Plate:      "RE05BKG",
		Road:       368,
		Mile1:      1234,
		Timestamp1: 1000000,
		Mile2:      1235,
		Timestamp2: 1000060,
		Speed:      6000,
	}},
	{mustDecodeHex("41"), &MsgHeartbeat{}},
}

type interposedReader struct {
	r io.Reader
}

func (r *interposedReader) Read(b []byte) (n int, err error) {
	n, err = r.r.Read(b)
	log.Printf("InterposedReader: n: %d, err: %s Read(%#v)\n", n, err, b)
	return n, err
}

func TestUnmarshalClientToServer(t *testing.T) {
	testUnmarshal(t, NewUnmarshaller(), unmarshalCases)
}

func TestUnmarshalServerToClient(t *testing.T) {
	testUnmarshal(t, NewMarshaller(), marshalCases)
}

func testUnmarshal(t *testing.T, unmarshaller *Unmarshaller, cases []UnmarshalCase) {
	for _, c := range cases {
		result, err := unmarshaller.UnmarshalMessage(&interposedReader{bytes.NewBuffer(c.Data)})
		if err != nil {
			t.Errorf("Failed to unmarshal data (got error %s) [case %+v]", err, c)
			continue
		}
		if !reflect.DeepEqual(result, c.ExpectedValue) {
			log.Println("types were", reflect.TypeOf(result), reflect.TypeOf(c.ExpectedValue))
			t.Errorf("Failed to unmarshal data (result does not match expected). Got %+v. [case %+v]",
				result, c)
			continue
		}
	}
}

func TestMarshalClientToServer(t *testing.T) {
	testMarshal(t, NewUnmarshaller(), unmarshalCases)
}

func TestMarshalServerToClient(t *testing.T) {
	testMarshal(t, NewMarshaller(), marshalCases)
}

func testMarshal(t *testing.T, unmarshaller *Unmarshaller, cases []UnmarshalCase) {
	for _, c := range cases {
		var buf bytes.Buffer
		err := unmarshaller.MarshalMessage(&buf, c.ExpectedValue)
		if err != nil {
			t.Errorf("Failed to marshal data (got error %s) [case %+v]", err, c)
			continue
		}
		if !bytes.Equal(buf.Bytes(), c.Data) {
			t.Errorf("Failed to marshal data (result does not match data). Got %+v. [case %+v]", buf.Bytes(), c)
			continue
		}
		log.Println("Case succeeded:", c)
	}
}
