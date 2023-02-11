package main

const (
    End = iota
    ReverseBits
    Xor
    Xorpos
    Add
    Addpos
    MaxCipher
)
var SecondByteNeeded = []bool {
    false,
    false,
    true,
    false,
    true,
    false,
}

type Cipher struct {
    Key byte
    Func CipherFunc
}
type CipherFunc func ([]byte, uint64, byte, bool)
func GetCipher(one, two byte) Cipher {
    return Cipher {
        Func: Ciphers[one],
        Key: two,
    }
}
var Ciphers = []CipherFunc{
    nil,
    ReverseBitsCipher,
    XorCipher,
    XorposCipher,
    AddCipher,
    AddposCipher,
}

func Crypt(ciphers []Cipher, data []byte, pos uint64, reverse bool) {
    for n := 0; n < len(ciphers); n++ {
        nn := n
        if reverse {
            nn = len(ciphers) - nn - 1
        }
        ciphers[nn].Func(data, pos, ciphers[nn].Key, reverse)
    }
}

func ReverseBitsCipher(data []byte, pos uint64, key byte, reverse bool) {
    for i, b := range data {
        newb := (b & 0x80) >> 7
        newb |= (b & 0x40) >> 5
        newb |= (b & 0x20) >> 3
        newb |= (b & 0x10) >> 1
        newb |= (b & 0x08) << 1
        newb |= (b & 0x04) << 3
        newb |= (b & 0x02) << 5
        newb |= (b & 0x01) << 7
        data[i] = newb
    }
}

func XorCipher(data []byte, pos uint64, key byte, reverse bool) {
    for i, b := range data {
        data[i] = b ^ key
    }
}

func XorposCipher(data []byte, pos uint64, key byte, reverse bool) {
    for i, b := range data {
        data[i] = b ^ (byte(pos) + byte(i))
    }
}

func addInverse(n byte) byte {
    return ^n + 1
}

func AddCipher(data []byte, pos uint64, key byte, reverse bool) {
    if reverse {
        key = addInverse(key)
    }
    for i, b := range data {
        data[i] = b + key
    }
}

func AddposCipher(data []byte, pos uint64, key byte, reverse bool) {
    for i, b := range data {
        k := byte(pos) + byte(i)
        if reverse {
            k = addInverse(k)
        }
        data[i] = b + k
    }
}

func isNoOpCipher(ciphers []Cipher) bool {
    buf := make([]byte, 1)
    for b := 0; b < 256; b++ {
        buf[0] = byte(b)
        Crypt(ciphers, buf, 1, false)
        if buf[0] != byte(b) {
            return false
        }
        buf[0] = byte(b)
        Crypt(ciphers, buf, 2, false)
        if buf[0] != byte(b) {
            return false
        }
    }
    return true
}
