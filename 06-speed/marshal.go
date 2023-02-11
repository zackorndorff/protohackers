package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"
)

type SerializedString struct {
	Len  uint8 `zmarshal:"length:Data"`
	Data []uint8
}

func (ss *SerializedString) String() string {
	// TODO make this actually ASCII
	return string(ss.Data)
}

// TODO: make more generic
func SerializeString(w io.Writer, us *UnmarshalState, s string) error {
	ss := &SerializedString{
		Len:  uint8(len(s)), // TODO: check overflow
		Data: []uint8(s),
	}
	return us.Marshal(w, reflect.Indirect(reflect.ValueOf(ss)))
}

type Unmarshaller struct {
	MessageTypes map[MsgType]reflect.Type
	Messages     map[reflect.Type]MsgType
	StringType   reflect.Type
}

var ErrCouldntWrite = errors.New("could not write full message to client")
var ErrCouldntRead = errors.New("could not read full message from client")
var ErrInvalidMsgType = errors.New("message type not valid")
var ErrInvalidLengthOf = errors.New("length tag was set on a non-uint field")
var ErrNoSizeForSlice = errors.New("no size provided for slice member of struct")

type UnmarshalState struct {
	KnownLengths      map[string]int
	KnownLengthFields map[string]reflect.Value
	Unmarshaller      *Unmarshaller
}

func (u *UnmarshalState) Unmarshal(r io.Reader, v reflect.Value) error {
	//log.Println("t was", v.Type())
	//log.Println("Unmarshalling kind", v.Type().Kind())
	switch v.Type().Kind() {
	//case reflect.Pointer:
	//	err := u.Unmarshal(r, v.Elem())
	//	if err != nil {
	//		return err
	//	}
	//	return nil

	case reflect.String:
		{
			//log.Println("string type is", u.Unmarshaller.StringType)
			data := reflect.New(u.Unmarshaller.StringType)
			err := u.Unmarshal(r, reflect.Indirect(data))
			if err != nil {
				return err
			}
			strVal := reflect.ValueOf(data.Interface().(fmt.Stringer).String())
			// Conversion necessary for typedefs
			v.Set(strVal.Convert(v.Type()))
			return nil
		}
	case reflect.Uint32:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint8:
		err := binary.Read(r, binary.BigEndian, v.Addr().Interface())
		if err != nil {
			//log.Println("error was", err)
			return ErrCouldntRead
		}
		return nil
	case reflect.Struct:
		fieldCount := v.NumField()
		newus := &UnmarshalState{
			KnownLengths: make(map[string]int),
			Unmarshaller: u.Unmarshaller,
		}
		for i := 0; i < fieldCount; i++ {
			f := v.Field(i)
			ftype := v.Type().Field(i)
			tag := ftype.Tag.Get("zmarshal")
			var lengthof string
			if strings.HasPrefix(tag, "length:") {
				lengthof = tag[len("length:"):]
				//log.Printf("this field %s is the length of %s", ftype.Name, lengthof)
				if !f.CanUint() {
					return ErrInvalidLengthOf
				}
			}
			var size *int
			if knownLen, ok := newus.KnownLengths[ftype.Name]; ok {
				size = &knownLen
			}
			// Initialize stuff if you need to
			switch f.Type().Kind() {
			case reflect.Slice:
				//log.Println("Making slice of type", reflect.SliceOf(f.Type().Elem()))
				f.Set(reflect.MakeSlice(reflect.SliceOf(f.Type().Elem()), *size, *size))
			default:
			}
			err := newus.Unmarshal(r, f)
			if err != nil {
				return err
			}
			if lengthof != "" {
				newus.KnownLengths[lengthof] = int(f.Uint())
				//log.Printf("Known length for %s is %d", lengthof, uint64(f.Uint()))
			}
		}
		return nil
	case reflect.Slice:
		fallthrough
	case reflect.Array:
		{
			for i := 0; i < v.Len(); i++ {
				err := u.Unmarshal(r, v.Index(i))
				if err != nil {
					return err
				}
			}
			return nil
		}
	default:
		return fmt.Errorf("unmarshal: unsupported kind %s (type %s)", v.Type().Kind(), v.Type())
	}
}

func (us *UnmarshalState) Marshal(w io.Writer, v reflect.Value) error {
	switch v.Type().Kind() {
	case reflect.Uint32:
		fallthrough
	case reflect.Uint16:
		fallthrough
	case reflect.Uint8:
		err := binary.Write(w, binary.BigEndian, v.Interface())
		if err != nil {
			return err
		}
		return nil
	case reflect.String:
		return SerializeString(w, us, v.String())
	case reflect.Struct:
		fieldCount := v.NumField()
		newus := &UnmarshalState{
			KnownLengthFields: make(map[string]reflect.Value),
			Unmarshaller:      us.Unmarshaller,
		}
		// We use a three-pass algorithm for structs to deal with known lengths
		// (we could shorten by a pass by noting fields as we see them; they
		// to be either before or after the length, and I think it works out)

		// First, find out which fields we care about
		for i := 0; i < fieldCount; i++ {
			f := v.Field(i)
			ftype := v.Type().Field(i)
			tag := ftype.Tag.Get("zmarshal")
			var lengthof string
			if strings.HasPrefix(tag, "length:") {
				lengthof = tag[len("length:"):]
				//log.Printf("this field %s is the length of %s", ftype.Name, lengthof)
				if !f.CanUint() {
					return ErrInvalidLengthOf
				}
				newus.KnownLengthFields[lengthof] = f
			}
		}

		// Second, find out the lengths of the relevant fields
		sType := v.Type()
		for i := 0; i < fieldCount; i++ {
			name := sType.Field(i).Name
			if _, ok := newus.KnownLengthFields[name]; ok {
				f := v.Field(i) // TODO check type
				// TODO: check overflow
				newus.KnownLengthFields[name].SetUint(uint64(f.Len()))
			}
		}

		// Third, loop over fields and serialize them
		for i := 0; i < fieldCount; i++ {
			err := newus.Marshal(w, v.Field(i))
			if err != nil {
				return err
			}
		}
		return nil
	case reflect.Array:
		fallthrough
	case reflect.Slice:
		for i := 0; i < v.Len(); i++ {
			err := us.Marshal(w, v.Index(i))
			if err != nil {
				return err
			}
		}
		return nil
	default:
		return fmt.Errorf("unsupported marshal kind %s (type %s)", v.Type().Kind(), v.Type())
	}
}
