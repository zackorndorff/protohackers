package main

type MsgType int

const (
	MsgTypeError         MsgType = 0x10
	MsgTypePlate         MsgType = 0x20
	MsgTypeTicket        MsgType = 0x21
	MsgTypeWantHeartbeat MsgType = 0x40
	MsgTypeHeartbeat     MsgType = 0x41
	MsgTypeIAmCamera     MsgType = 0x80
	MsgTypeIAmDispatcher MsgType = 0x81
)

type MsgError struct {
	Msg string
}

type MsgPlate struct {
	Plate     string
	Timestamp uint32
}

type MsgWantHeartbeat struct {
	Interval uint32
}

type MsgHeartbeat struct {
}

type MsgIAmCamera struct {
	Road  uint16
	Mile  uint16
	Limit uint16 // mph
}

type MsgIAmDispatcher struct {
	NumRoads uint8 `zmarshal:"length:Roads"`
	Roads    []uint16
}
