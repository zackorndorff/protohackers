package protocol

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"reflect"
)

type MsgType byte

// checksum code
func sumData(data []byte) byte {
	accum := byte(0)
	for _, i := range data {
		accum += i
	}
	return accum
}

func UpdateChecksum(data []byte) {
	last := len(data) - 1
	data[last] = 0
	data[last] = -sumData(data)
}

func ValidateChecksum(data []byte) bool {
	return sumData(data) == 0
}

func NewUnmarshallerForTesting() *Unmarshaller {
	types := make(map[MsgType]reflect.Type)
	types[MsgTypeHello] = reflect.TypeOf((*MsgHello)(nil)).Elem()
	types[MsgTypeError] = reflect.TypeOf((*MsgError)(nil)).Elem()
	types[MsgTypeOk] = reflect.TypeOf((*MsgOk)(nil)).Elem()
	types[MsgTypeDialAuthority] = reflect.TypeOf((*MsgDialAuthority)(nil)).Elem()
	types[MsgTypeTargetPopulations] = reflect.TypeOf((*MsgTargetPopulations)(nil)).Elem()
	types[MsgTypeCreatePolicy] = reflect.TypeOf((*MsgCreatePolicy)(nil)).Elem()
	types[MsgTypeDeletePolicy] = reflect.TypeOf((*MsgDeletePolicy)(nil)).Elem()
	types[MsgTypePolicyResult] = reflect.TypeOf((*MsgPolicyResult)(nil)).Elem()
	types[MsgTypeSiteVisit] = reflect.TypeOf((*MsgSiteVisit)(nil)).Elem()

	return newUnmarshaller(types)
}

func NewUnmarshallerForServerEnd() *Unmarshaller {
	types := make(map[MsgType]reflect.Type)
	types[MsgTypeHello] = reflect.TypeOf((*MsgHello)(nil)).Elem()
	types[MsgTypeSiteVisit] = reflect.TypeOf((*MsgSiteVisit)(nil)).Elem()

	return newUnmarshaller(types)
}

func newUnmarshaller(types map[MsgType]reflect.Type) *Unmarshaller {
	messages := make(map[reflect.Type]MsgType)
	for k, v := range types {
		messages[v] = k
	}

	return &Unmarshaller{
		MessageTypes: types,
		StringType:   reflect.TypeOf((*SerializedString)(nil)).Elem(),
		Messages:     messages,
	}
}

var ErrBadLength = errors.New("invalid length specified")
var ErrTooBig = errors.New("too long of message")
var ErrInvalidChecksum = errors.New("invalid checksum")
var ErrTooShort = errors.New("message does not take up the size the length said it did")

// Returns pointer to unmarshalled thing
func (u *Unmarshaller) UnmarshalMessage(r io.Reader) (interface{}, error) {
	const HeaderLength = 5
	header := make([]byte, HeaderLength)
	_, err := io.ReadFull(r, header)
	if err != nil {
		log.Println("error ReadFull:", err)
		return nil, ErrCouldntRead
	}

	headerdup := make([]byte, len(header))
	copy(headerdup, header)
	headerbuf := bytes.NewBuffer(headerdup)

	var msgType uint8
	err = binary.Read(headerbuf, binary.BigEndian, &msgType)
	if err != nil {
		log.Println(err)
		return nil, ErrCouldntRead
	}

	var msgLen uint32
	err = binary.Read(headerbuf, binary.BigEndian, &msgLen)
	if err != nil {
		log.Println(err)
		return nil, ErrCouldntRead
	}
	log.Println("proposed len is", msgLen)
	if msgLen < HeaderLength {
		return nil, ErrBadLength
	}
	msgLen -= HeaderLength

	if msgLen > 0x100000 {
		return nil, ErrTooBig
	}

	msg := make([]byte, msgLen)
	_, err = io.ReadFull(r, msg)
	if err != nil {
		log.Println("reading msg", err)
		return nil, ErrCouldntRead
	}

	checksumbuf := bytes.NewBuffer(header)
	_, err = checksumbuf.Write(msg)
	if err != nil {
		return nil, err
	}

	if !ValidateChecksum(headerbuf.Bytes()) {
		return nil, ErrInvalidChecksum
	}

	msgStructType, ok := u.MessageTypes[MsgType(msgType)]
	if !ok {
		return nil, ErrInvalidMsgType
	}

	us := &UnmarshalState{
		Unmarshaller: u,
	}

	val := reflect.New(msgStructType)
	msgr := bytes.NewReader(msg)
	err = us.Unmarshal(msgr, reflect.Indirect(val))
	if err != nil {
		return nil, err
	}
	if msgr.Len() != 1 { // 1 for checksum
		return nil, ErrTooShort
	}
	return val.Interface(), nil
}

func (u *Unmarshaller) MarshalMessage(w io.Writer, data interface{}) error {
	var buf bytes.Buffer

	msgtype, ok := u.Messages[reflect.TypeOf(data).Elem()]
	if !ok {
		return ErrInvalidMsgType
	}

	err := binary.Write(&buf, binary.BigEndian, uint8(msgtype))
	if err != nil {
		return ErrCouldntWrite
	}

	// length, to be filled in later
	err = binary.Write(&buf, binary.BigEndian, uint32(0))
	if err != nil {
		return ErrCouldntWrite
	}

	us := &UnmarshalState{
		Unmarshaller: u,
	}

	err = us.Marshal(&buf, reflect.ValueOf(data).Elem())
	if err != nil {
		return fmt.Errorf("idk we failed: %w", err)
	}

	err = binary.Write(&buf, binary.BigEndian, uint8(0))
	if err != nil {
		return ErrCouldntWrite
	}

	var lengthbuf bytes.Buffer
	err = binary.Write(&lengthbuf, binary.BigEndian, uint32(buf.Len()))
	if err != nil {
		panic(err)
	}

	bufbuf := buf.Bytes()
	for i, b := range lengthbuf.Bytes() {
		bufbuf[i+1] = b
	}

	UpdateChecksum(buf.Bytes())

	_, err = buf.WriteTo(w)
	if err != nil {
		return ErrCouldntWrite
	}
	return nil
}
