package main

import (
    "bytes"
    "reflect"
    "testing"
)

type IsNoOpCase struct {
    Data string
    Result bool
}
var isNoOpCases []IsNoOpCase = []IsNoOpCase {
    {"0100", false},
    {"050500", false},
    {"02010100", false},
    {"027b050100", false},
    {"020000", true},
    {"02ab02ab00", true},
    {"010100", true},
    {"02a0020b02ab00", true},
}

func TestIsNoOp(t *testing.T) {
    for _, c := range isNoOpCases {
        buf := bytes.NewBuffer(decodeHex(t, c.Data))
        ciphers, err := parseHandshake(buf)
        if err != nil {
            t.Fatalf("Bad case, failed to parse %s", c.Data)
        }
        res := isNoOpCipher(ciphers) 
        if res != c.Result {
            t.Fatalf("Failed. Got %v, expected %v (%#v)", res, c.Result, c.Data)
        }
    }
}

func TestAddInverse(t *testing.T) {
    for i := 0; i < 256; i++ {
        inv := addInverse(byte(i))
        res := byte(i) + inv 
        if res != 0 {
            t.Fatalf("addInverse(%d) = %d is wrong (%d)", i, inv, res)
        }
    }
}
func TestMoreAddInverse(t *testing.T) {
    for i := 0; i < 256; i++ {
        inv := addInverse(byte(i))
        res := byte(i) + inv 
        if res != 0 {
            t.Fatalf("addInverse(%d) = %d is wrong (%d)", i, inv, res)
        }
        for b := 0; b < 256; b++ {
            v := byte(b)
            v += byte(i)
            v += inv
            if byte(b) != v {
                t.Errorf("the inverse for %d doesn't work [orig] %d + [key]%d + [inv]%d == [not orig]%d", i, b, i, inv, v)
            }
        }
    }
}

func TestCipherListSmoke(t *testing.T) {
    //addpos,xor(20),reversebits,xorpos,reversebits,reversebits
    ciphers := []Cipher {
        Cipher{ Func: AddposCipher},
        Cipher{ Func: XorCipher, Key: 20},
        Cipher{ Func: ReverseBitsCipher},
        Cipher{ Func: XorposCipher},
        Cipher{ Func: ReverseBitsCipher},
        Cipher{ Func: ReverseBitsCipher},
    }
    testbuf := []byte("hello world the night is young\n")
    pos := uint64(792)
    buf := make([]byte, len(testbuf))
    copy(buf, testbuf)
    Crypt(ciphers, testbuf, pos, false)
    Crypt(ciphers, testbuf, uint64(byte(pos)), true)
    if !reflect.DeepEqual(testbuf, buf) {
        t.Errorf("cipher does not work %v %v", buf, testbuf)
    }
}

func TestCiphersSmoke(t *testing.T) {
    testbuf := []byte("hello world the night is young\n")
    key := byte(42)
    pos := uint64(792)
    buf := make([]byte, len(testbuf))
    for i := End + 1; i < MaxCipher; i++ {
        copy(buf, testbuf)
        Ciphers[i](buf, pos, key, false)
        Ciphers[i](buf, pos, key, true)
        if !reflect.DeepEqual(testbuf, buf) {
            t.Errorf("cipher %d does not work %v %v", i, buf, testbuf)
        }
    }
}

type ReverseBitsCase struct {
    In, Out []byte
}

var reverseBitsCases []ReverseBitsCase = []ReverseBitsCase{
    {[]byte{1, 1}, []byte{0x80, 0x80}},
    {[]byte{2}, []byte{0x40}},
}

func TestReverseBits(t *testing.T) {
    for _, b := range reverseBitsCases {
        testbuf := make([]byte, len(b.In))
        copy(testbuf, b.In)
        ReverseBitsCipher(testbuf, 0, 0, false)
        if !reflect.DeepEqual(testbuf, b.Out) {
            t.Errorf("[%v] Not equal: %v | %v", b.In, testbuf, b.Out)
        }
    }
}
