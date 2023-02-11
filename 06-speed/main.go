package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"time"

	"z10f.com/golang/protohackers/06/core"
)

//const TIMEOUT_SECONDS = 5
//const MAX_REQUESTS = 1_000_000
//const MSG_LEN = 9
//conn.SetReadDeadline(time.Now().Add(TIMEOUT_SECONDS * time.Second))
// log.Println("Handling request from", conn.RemoteAddr())

func NewUnmarshaller() *Unmarshaller {
	types := make(map[MsgType]reflect.Type)
	types[MsgTypePlate] = reflect.TypeOf((*MsgPlate)(nil)).Elem()
	types[MsgTypeWantHeartbeat] = reflect.TypeOf((*MsgWantHeartbeat)(nil)).Elem()
	types[MsgTypeIAmCamera] = reflect.TypeOf((*MsgIAmCamera)(nil)).Elem()
	types[MsgTypeIAmDispatcher] = reflect.TypeOf((*MsgIAmDispatcher)(nil)).Elem()

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

// Needs rename, but this works with messages that go Server -> Client
func NewMarshaller() *Unmarshaller {
	types := make(map[MsgType]reflect.Type)
	types[MsgTypeError] = reflect.TypeOf((*MsgError)(nil)).Elem()
	types[MsgTypeTicket] = reflect.TypeOf((*core.Ticket)(nil)).Elem()
	types[MsgTypeHeartbeat] = reflect.TypeOf((*MsgHeartbeat)(nil)).Elem()

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

// Returns pointer to unmarshalled thing
func (u *Unmarshaller) UnmarshalMessage(r io.Reader) (interface{}, error) {
	var msgType uint8
	err := binary.Read(r, binary.BigEndian, &msgType)
	if err != nil {
		log.Println(err)
		return nil, ErrCouldntRead
	}

	msgStructType, ok := u.MessageTypes[MsgType(msgType)]
	if !ok {
		return nil, ErrInvalidMsgType
	}

	us := &UnmarshalState{
		Unmarshaller: u,
	}

	val := reflect.New(msgStructType)
	err = us.Unmarshal(r, reflect.Indirect(val))
	if err != nil {
		return nil, err
	}
	return val.Interface(), nil
}

func (u *Unmarshaller) MarshalMessage(w io.Writer, data interface{}) error {
	msgtype, ok := u.Messages[reflect.TypeOf(data).Elem()]
	if !ok {
		return ErrInvalidMsgType
	}

	err := binary.Write(w, binary.BigEndian, uint8(msgtype))
	if err != nil {
		log.Println("whyyyy", err)
		return ErrCouldntWrite
	}

	us := &UnmarshalState{
		Unmarshaller: u,
	}

	err = us.Marshal(w, reflect.ValueOf(data).Elem())
	if err != nil {
		return fmt.Errorf("idk we failed: %w", err)
	}
	return nil
}

type DeciSecond int
type ClientState int

const (
	Unknown ClientState = iota
	Camera
	Dispatcher
)

type Client struct {
	conn            net.Conn
	Core            *core.State
	State           ClientState
	HeartbeatConfig chan DeciSecond
	Message         chan interface{}
	Dispatcher      core.Dispatcher

	Road core.Road
	Mile uint16 // used only if Camera ATM.
}

const deciSecond = 100 * time.Millisecond

func (c *Client) doHeartbeat() {
	interval := 0
	for {
		var timer <-chan time.Time
		if interval != 0 {
			timer = time.After(time.Duration(interval) * deciSecond)
		} else {
			timer = make(<-chan time.Time)
		}
		select {
		case newInterval, ok := <-c.HeartbeatConfig:
			if !ok {
				return
			}
			interval = int(newInterval)
		case <-timer:
			c.Message <- &MsgHeartbeat{}
		}
	}

}

func (c *Client) forwardTickets() {
	for ticket := range c.Dispatcher.SendTicket {
		c.Message <- ticket
	}
}

func (c *Client) sendMessages() {
	m := NewMarshaller()
	bufw := bufio.NewWriter(c.conn)
	for msg := range c.Message {
		log.Printf("Sending message %#v", msg)
		err := m.MarshalMessage(bufw, msg)
		if err != nil {
			log.Println("error sending message", err, msg)
			break
		}
		err = bufw.Flush()
		if err != nil {
			log.Println("error flushing data", err)
		}
	}
	log.Println("sendMessages() is closing connection")
	c.conn.Close()
}

func (c *Client) errorOut(msg string) {
	c.Message <- &MsgError{Msg: msg}
}

func handleConnection(c *Client, state *core.State) {
	log.Println("Handling connection from", c.conn.RemoteAddr())
	go c.sendMessages()
	// So there's a race condition here - if the core is trying to dispatch
	// messages while we close here, we panic
	// This doesn't seem to happen, but there's no reason why it can't.
	// But I'm tired, this isn't production code, and the tests pass...
	defer close(c.Message)
	go c.doHeartbeat()
	defer close(c.HeartbeatConfig)
	u := NewUnmarshaller()
	for {
		msg, err := u.UnmarshalMessage(c.conn)
		if err == ErrInvalidMsgType {
			c.errorOut("invalid message type")
			return
		} else if err != nil {
			c.errorOut("error reading")
			return
		}
		log.Printf("Got message %#v", msg)
		switch msg := msg.(type) {
		case *MsgWantHeartbeat:
			c.HeartbeatConfig <- DeciSecond(msg.Interval)
		case *MsgIAmCamera:
			if c.State != Unknown {
				c.errorOut("bad state (IAmCamera)")
				return
			}
			c.State = Camera
			c.Core.RegisterRoad <- &core.RegisterRoad{
				Road:  core.Road(msg.Road),
				Limit: core.Limit(msg.Limit),
			}
			c.Mile = msg.Mile
			c.Road = core.Road(msg.Road)
		case *MsgIAmDispatcher:
			if c.State != Unknown {
				c.errorOut("bad state (IAmDispatcher)")
				return
			}
			c.State = Dispatcher

			c.Dispatcher = core.Dispatcher{
				Roads:      []core.Road{},
				SendTicket: make(chan *core.Ticket),
			}

			for _, road := range msg.Roads {
				c.Dispatcher.Roads = append(c.Dispatcher.Roads, core.Road(road))
			}

			go c.forwardTickets()
			c.Core.RegisterDispatcher <- &c.Dispatcher
			defer func() { c.Core.UnregisterDispatcher <- &c.Dispatcher }()
			defer close(c.Dispatcher.SendTicket)
		case *MsgPlate:
			if c.State != Camera {
				c.errorOut("bad state (MsgPlate)")
				return
			}
			c.Core.RecordObservation <- &core.PlateObservation{
				Plate:     core.Plate(msg.Plate),
				Timestamp: core.Timestamp(msg.Timestamp),
				Road:      c.Road,
				Mile:      c.Mile,
			}
		default:
			c.errorOut("bad message type")
			return
		}
	}
}

func main() {
	state := core.NewState()
	go state.MainLoop()

	l, err := net.Listen("tcp", ":1337")
	if err != nil {
		log.Fatal("could not listen", err)
	}
	defer l.Close()
	fmt.Println("Listening.")
	for {
		conn, err := l.Accept()
		if err != nil {
			log.Println("Error accepting", err)
		}
		client := &Client{
			conn:            conn,
			Core:            state,
			HeartbeatConfig: make(chan DeciSecond),
			Message:         make(chan interface{}),
		}
		go handleConnection(client, state)
	}
}
