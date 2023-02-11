package protocol

import (
	"bytes"
	"encoding/hex"
	"io"
	"log"
	"reflect"
	"strings"
	"testing"
)

func mustDecodeHex(hexdata string) []byte {
	hexdata = strings.ReplaceAll(hexdata, " ", "")
	data, err := hex.DecodeString(hexdata)
	if err != nil {
		panic(err)
	}
	return data
}

type ChecksumCase struct {
	Data  []byte
	Valid bool
}

var checksumCases []ChecksumCase = []ChecksumCase{
	{mustDecodeHex("52 00 00 00 06 a8"), true},
	{mustDecodeHex("53 00 00 00 0a 00 00 30 39 3a"), true},
	{mustDecodeHex("01 00"), false},
}

func TestChecksum(t *testing.T) {
	for i, c := range checksumCases {
		if c.Valid {
			if !ValidateChecksum(c.Data) {
				t.Errorf("Failed to validate correct checksum (test case %d)", i)
			}
		}
		tmp := make([]byte, len(c.Data))
		copy(tmp, c.Data)
		last := len(tmp) - 1
		if tmp[last] != 0 {
			tmp[last] = 0
		} else {
			tmp[last] = 1
		}

		UpdateChecksum(tmp)

		if c.Valid {
			if !reflect.DeepEqual(tmp, c.Data) {
				t.Errorf("Failed to generate correct checksum (test case %d)", i)
			}
		}

		if !ValidateChecksum(tmp) {
			t.Errorf("Checksum did not validate after update (test case %d)", i)
		}
	}
}

type UnmarshalCase struct {
	Data          []byte
	ExpectedValue interface{}
}

var unmarshalCases []UnmarshalCase = []UnmarshalCase{
	{mustDecodeHex("50 00 00 00 19 00 00 00 0b 70 65 73 74 63 6f 6e 74 72 6f 6c 00 00 00 01 ce"),
		&MsgHello{Protocol: "pestcontrol", Version: 1}},
	{mustDecodeHex("51 00 00 00 0d 00 00 00 03 62 61 64 78"), &MsgError{Message: "bad"}},
	{mustDecodeHex("52 00 00 00 06 a8"), &MsgOk{}},
	{mustDecodeHex("53 00 00 00 0a 00 00 30 39 3a"), &MsgDialAuthority{Site: 12345}},
	{mustDecodeHex("54 00 00 00 2c 00 00 30 39 00 00 00 02 00 00 00 03 64 6f 67 00 00 00 01 00 00 00 03 00 00 00 03 72 61 74 00 00 00 00 00 00 00 0a 80"),
		&MsgTargetPopulations{Site: 12345, PopulationCount: 2, Populations: []TargetPopulation{
			{"dog", 1, 3},
			{"rat", 0, 10},
		}}},
	{mustDecodeHex("55 00 00 00 0e 00 00 00 03 64 6f 67 a0 c0"), &MsgCreatePolicy{Species: "dog", Action: ActionConserve}},
	{mustDecodeHex("56 00 00 00 0a 00 00 00 7b 25"), &MsgDeletePolicy{Policy: 123}},
	{mustDecodeHex("57 00 00 00 0a 00 00 00 7b 24"), &MsgPolicyResult{Policy: 123}},
	{mustDecodeHex("58 00 00 00 24 00 00 30 39 00 00 00 02 00 00 00 03 64 6f 67 00 00 00 01 00 00 00 03 72 61 74 00 00 00 05 8c"),
		&MsgSiteVisit{Site: 12345, PopulationCount: 2, Populations: []SitePopulation{
			{Species: "dog", Count: 1},
			{Species: "rat", Count: 5},
		}}},
}

// var marshalCases []UnmarshalCase = []UnmarshalCase{
// 	{mustDecodeHex("10 03 62 61 64"), &MsgError{Msg: "bad"}},
// 	{mustDecodeHex("10 0b 69 6c 6c 65 67 61 6c 20 6d 73 67"), &MsgError{Msg: "illegal msg"}},
// 	{mustDecodeHex("21 04 55 4e 31 58 00 42 00 64 00 01 e2 40 00 6e 00 01 e3 a8 27 10"), &core.Ticket{
// 		Plate:      "UN1X",
// 		Road:       66,
// 		Mile1:      100,
// 		Timestamp1: 123456,
// 		Mile2:      110,
// 		Timestamp2: 123816,
// 		Speed:      10000,
// 	}},
// 	{mustDecodeHex("21 07 52 45 30 35 42 4b 47 01 70 04 d2 00 0f 42 40 04 d3 00 0f 42 7c 17 70"), &core.Ticket{
// 		Plate:      "RE05BKG",
// 		Road:       368,
// 		Mile1:      1234,
// 		Timestamp1: 1000000,
// 		Mile2:      1235,
// 		Timestamp2: 1000060,
// 		Speed:      6000,
// 	}},
// 	{mustDecodeHex("41"), &MsgHeartbeat{}},
// }

type interposedReader struct {
	r io.Reader
}

func (r *interposedReader) Read(b []byte) (n int, err error) {
	n, err = r.r.Read(b)
	log.Printf("InterposedReader: n: %d, err: %s Read(%#v)\n", n, err, b)
	return n, err
}

func TestUnmarshal(t *testing.T) {
	testUnmarshal(t, NewUnmarshallerForTesting(), unmarshalCases)
}

func TestMarshal(t *testing.T) {
	testMarshal(t, NewUnmarshallerForTesting(), unmarshalCases)
}

// func TestUnmarshalClientToServer(t *testing.T) {
// 	testUnmarshal(t, NewUnmarshaller(), unmarshalCases)
// }

// func TestUnmarshalServerToClient(t *testing.T) {
// 	testUnmarshal(t, NewMarshaller(), marshalCases)
// }

func testUnmarshal(t *testing.T, unmarshaller *Unmarshaller, cases []UnmarshalCase) {
	for _, c := range cases {
		if !ValidateChecksum(c.Data) {
			t.Errorf("checksum was invalid")
		}
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

// func TestMarshalClientToServer(t *testing.T) {
// 	testMarshal(t, NewUnmarshaller(), unmarshalCases)
// }

// func TestMarshalServerToClient(t *testing.T) {
// 	testMarshal(t, NewMarshaller(), marshalCases)
// }

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
