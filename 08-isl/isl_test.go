package main

import (
    "bytes"
    "encoding/hex"
    "io"
    "net"
    "reflect"
    "strings"
    "testing"
    "time"
)

func decodeHex(t *testing.T, hexdata string) []byte {
	hexdata = strings.ReplaceAll(hexdata, " ", "")
	data, err := hex.DecodeString(hexdata)
	if err != nil {
            t.Fatalf("error decoding hex")
	}
	return data
}
type ParseCipherCase struct {
    Data string
    Result []Cipher
}

var parseCipherCases []ParseCipherCase = []ParseCipherCase {
    {"0100", []Cipher{
        Cipher{Func:ReverseBitsCipher},
    }},
    {"050500", []Cipher{
        Cipher{Func:AddposCipher},
        Cipher{Func:AddposCipher},
    }},
    {"02010100", []Cipher{
        Cipher{Func: XorCipher, Key: 1},
        Cipher{Func:ReverseBitsCipher},
    }},
    {"027b050100", []Cipher{
        Cipher{Func: XorCipher, Key: 123},
        Cipher{Func: AddposCipher},
        Cipher{Func:ReverseBitsCipher},
    }},
}

func minInt(a, b int) int {
    if a < b {
        return a
    } else {
        return b
    }
}

func TestParseHandshake(t *testing.T) {
    for _, c := range parseCipherCases {
        buf := bytes.NewBuffer(decodeHex(t, c.Data))
        ciphers, err := parseHandshake(buf)
        if err != nil {
            t.Fatalf("Error parsing %s %#v", err, c)
        }
        good := true
        if len(ciphers) != len(c.Result) {
            good = false
        }
        for i := 0; i < minInt(len(ciphers), len(c.Result)); i++ {
            if reflect.ValueOf(ciphers[i].Func).Pointer() != reflect.ValueOf(c.Result[i].Func).Pointer() {
                good = false
                break
            }
            if ciphers[i].Key != c.Result[i].Key {
                good = false
                break
            }
        }
        if !good {
            t.Fatalf("Bad: \n\tgot      %#v,\n\texpected %#v", ciphers, c.Result)
        }
    }
}

type FakeConn struct {
    buf bytes.Buffer
}

func (fc *FakeConn) Close() error {
    return nil
}
func (fc *FakeConn) LocalAddr() net.Addr {
    panic("buzz off")
}
func (fc *FakeConn) RemoteAddr() net.Addr {
    panic("buzz off")
}
func (fc *FakeConn) Read(buf []byte) (int, error) {
    return fc.buf.Read(buf)
}
func (fc *FakeConn) Write(buf []byte) (int, error) {
    return fc.buf.Write(buf)
}
func (fc *FakeConn) SetDeadline(t time.Time) error {
    panic("buzz off")
}
func (fc *FakeConn) SetReadDeadline(t time.Time) error {
    panic("buzz off")
}
func (fc *FakeConn) SetWriteDeadline(t time.Time) error {
    panic("buzz off")
}

func TestSession(t *testing.T) {
    buf := bytes.Buffer{}
    buf.Write(decodeHex(t, "02 7b 05 01 00"))
    parsed, err := parseHandshake(&buf)
    if err != nil {
        t.Fatalf("Error parsing handshake %s", err)
    }
    buf.Reset()
    buf.Write(decodeHex(t, "f2 20 ba 44 18 84 ba aa d0 26 44 a4 a8 7e"))
    fc := &FakeConn {
        buf,
    }
    c := &Conn {
        Conn: fc,
        ciphers: parsed,
    }

    data, _ := io.ReadAll(c)
    should_be := []byte("4x dog,5x car\n")
    if !reflect.DeepEqual(data, should_be) {
        t.Fatal(hex.EncodeToString(data), "||", string(data))
    }
}
