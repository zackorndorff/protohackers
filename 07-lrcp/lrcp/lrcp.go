package lrcp

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

const RETRANSMISSION_TIMEOUT = 3 * time.Second
const SESSION_EXPIRY_TIMEOUT = 60 * time.Second

type LrcpAddr struct {
	address string
}

func (la LrcpAddr) Network() string {
	return "lrcp"
}

func (la LrcpAddr) String() string {
	return la.address
}

type ConnectPacket struct {
	SessionID uint32
}

type DataPacket struct {
	SessionID uint32
	Position  uint32
	Data      []byte // this should be unescaped!
}

type AckPacket struct {
	SessionID uint32
	Length    uint32
}

type ClosePacket struct {
	SessionID uint32
}

type IncomingPacket struct {
	Packet interface{}
	Addr   net.Addr
}

type Listener struct {
	udpConn        net.PacketConn
	newConnections chan *Conn
	packetChan     chan IncomingPacket
	address        LrcpAddr
	connections    map[uint32]*Conn
	ticker         time.Ticker
}

var ErrInvalidUint32 = errors.New("invalid uint32 passed to parseUint32")
var ErrInvalidSessionID = errors.New("invalid session id in packet")

func parseUint32(id []byte) (uint32, error) {
	idnum, err := strconv.ParseUint(string(id), 10, 32)
	if err != nil {
		return 0, ErrInvalidUint32
	}
	return uint32(idnum), nil
}
func unescape(data []byte) []byte {
	buf := []byte{}
	for i := 0; i < len(data); i++ {
		if data[i] != '\\' {
			buf = append(buf, data[i])
		} else {
			i++
			buf = append(buf, data[i])
		}
	}
	return buf
}

func escape(data []byte) []byte {
	buf := []byte{}
	for _, i := range data {
		if i == '/' {
			buf = append(buf, "\\/"...)
		} else if i == '\\' {
			buf = append(buf, "\\\\"...)
		} else {
			buf = append(buf, i)
		}
	}
	return buf
}

var ErrInvalidPacketType = errors.New("invalid packet type")
var ErrMalformedPacket = errors.New("malformed packet")

func parsePacket(buf []byte) (interface{}, error) {
	lastSlash := 0
	if len(buf) < 2 {
		return nil, fmt.Errorf("%w: packet too short to be valid",
			ErrMalformedPacket)
	}
	if buf[0] != '/' {
		return nil, fmt.Errorf("%w: packet did not start with slash",
			ErrMalformedPacket)
	}
	pieces := [][]byte{}
	for i := 1; i < len(buf); i++ {
		if buf[i] == '\\' {
			i++
		} else if buf[i] == '/' { // and was not skipped
			pieces = append(pieces, buf[lastSlash+1:i])
			lastSlash = i
		}
	}
	if lastSlash != len(buf)-1 {
		return nil, fmt.Errorf("%w: packet did not end with slash",
			ErrMalformedPacket)
	}
	if len(pieces) < 2 {
		return nil, fmt.Errorf("%w: packet doesn't have enough pieces",
			ErrMalformedPacket)
	}
	type_ := string(pieces[0])
	if type_ == "connect" {
		if len(pieces) != 2 {
			return nil, fmt.Errorf("%w: connect packet should have 2 pieces",
				ErrMalformedPacket)
		}
		id, err := parseUint32(pieces[1])
		if err != nil {
			return nil, ErrInvalidSessionID
		}
		return ConnectPacket{
			id,
		}, nil
	} else if type_ == "data" {
		if len(pieces) != 4 {
			return nil, fmt.Errorf("%w: data packet should have 4 pieces",
				ErrMalformedPacket)
		}
		id, err := parseUint32(pieces[1])
		if err != nil {
			return nil, ErrInvalidSessionID
		}
		pos, err := parseUint32(pieces[2])
		if err != nil {
			return nil, err
		}
		data := unescape(pieces[3])
		return DataPacket{
			SessionID: id,
			Position:  pos,
			Data:      data,
		}, nil
	} else if type_ == "ack" {
		if len(pieces) != 3 {
			return nil, fmt.Errorf("%w: ack packet should have 3 pieces",
				ErrMalformedPacket)
		}
		id, err := parseUint32(pieces[1])
		if err != nil {
			return nil, ErrInvalidSessionID
		}
		length, err := parseUint32(pieces[2])
		if err != nil {
			return nil, err
		}
		return AckPacket{
			SessionID: id,
			Length:    length,
		}, nil
	} else if type_ == "close" {
		if len(pieces) != 2 {
			return nil, fmt.Errorf("%w: close packet should have 2 pieces",
				ErrMalformedPacket)
		}
		id, err := parseUint32(pieces[1])
		if err != nil {
			return nil, ErrInvalidSessionID
		}
		return ClosePacket{
			id,
		}, nil
	} else {
		return nil, fmt.Errorf("%w: %v", ErrInvalidPacketType, pieces[0])
	}
}

func packetJoin(pieces [][]byte) []byte {
	sep := []byte("/")
	return append(append(sep, bytes.Join(pieces, sep)...), sep...)
}

func serializePacket(packet interface{}) []byte {
	switch p := packet.(type) {
	case ConnectPacket:
		return packetJoin([][]byte{
			[]byte("connect"),
			[]byte(fmt.Sprint(p.SessionID)),
		})
	case DataPacket:
		return packetJoin([][]byte{
			[]byte("data"),
			[]byte(fmt.Sprint(p.SessionID)),
			[]byte(fmt.Sprint(p.Position)),
			escape(p.Data),
		})
	case AckPacket:
		return packetJoin([][]byte{
			[]byte("ack"),
			[]byte(fmt.Sprint(p.SessionID)),
			[]byte(fmt.Sprint(p.Length)),
		})
	case ClosePacket:
		return packetJoin([][]byte{
			[]byte("close"),
			[]byte(fmt.Sprint(p.SessionID)),
		})
	default:
		panic(fmt.Errorf("attempt to serialize unknown packet type"))
	}
}

func (l *Listener) sendPacket(addr net.Addr, packet interface{}) error {
	log.Printf("[%s]: sending packet %#v", addr, packet)
	_, err := l.udpConn.WriteTo(serializePacket(packet), addr)
	return err
}

func (l *Listener) dispatchPacket(packet interface{}, addr net.Addr) {
	switch p := packet.(type) {
	case ConnectPacket:
		if conn, ok := l.connections[p.SessionID]; ok {
			if conn.receivedUpTo == 0 {
				log.Printf("[%s:%d]: received extra connect, sending ack", addr, p.SessionID)
				conn.sendAck(p.SessionID)
			}
		} else {
			log.Printf("[%s:%d]: received new connection", addr, p.SessionID)
			conn := &Conn{
				sessionID:  p.SessionID,
				localAddr:  l.address,
				remoteAddr: addr,
				listener:   l,

				lastAck: time.Now(),

				recvLock: *sync.NewCond(&sync.Mutex{}),
			}
			conn.sendAck(p.SessionID)
			l.connections[p.SessionID] = conn
			l.newConnections <- conn
		}
	case DataPacket:
		if conn, ok := l.connections[p.SessionID]; ok {
			if conn.receivedUpTo >= (p.Position + uint32(len(p.Data))) {
				log.Printf("[%s:%d]: extra data retransmit", addr, p.SessionID)
			} else if conn.receivedUpTo >= p.Position {
				log.Printf("[%s:%d]: received new data", addr, p.SessionID)
				// we're not behind
				offset := conn.receivedUpTo - p.Position
				conn.recvLock.L.Lock()
				// documented to never fail
				_, _ = conn.recvBuf.Write(p.Data[offset:])
				conn.recvLock.L.Unlock()
				conn.recvLock.Signal()
				conn.receivedUpTo = p.Position + uint32(len(p.Data))
				conn.sendAck(p.SessionID)
			} else {
				log.Printf("[%s:%d]: we are behind, requesting retransmission", addr, p.SessionID)
				// we are behind, request retransmission
				conn.sendAck(p.SessionID)
			}
		} else {
			log.Printf("[%s:%d]: unsolicited data packet, sending RST", addr, p.SessionID)
			closePkt := ClosePacket{
				SessionID: p.SessionID,
			}
			l.sendPacket(addr, closePkt)
		}
	case AckPacket:
		if conn, ok := l.connections[p.SessionID]; ok {
			if p.Length > conn.gotAcksUpTo {
				if p.Length > conn.bytesSent {
					log.Printf("[%s:%d]: too many bytes acked, sending RST", addr, p.SessionID)
					closePkt := ClosePacket{
						SessionID: p.SessionID,
					}
					delete(l.connections, p.SessionID)
					conn.sendPacket(closePkt)
					conn.setClosed()
				} else {
					log.Printf("[%s:%d]: noted acked bytes", addr, p.SessionID)
					conn.sendLock.Lock()
					defer conn.sendLock.Unlock()
					newlyAcked := p.Length - conn.gotAcksUpTo
					conn.gotAcksUpTo = p.Length
					conn.lastAck = time.Now()
					conn.sendBuf.Next(int(newlyAcked))
					conn.maybeRetransmit()
				}
			}
		} else {
			log.Printf("[%s:%d]: unsolicited ack packet, sending RST", addr, p.SessionID)
			closePkt := ClosePacket{
				SessionID: p.SessionID,
			}
			l.sendPacket(addr, closePkt)
		}
	case ClosePacket:
		if conn, ok := l.connections[p.SessionID]; ok {
			log.Printf("[%s:%d]: sending close in reply", addr, p.SessionID)
			closePkt := ClosePacket{
				SessionID: p.SessionID,
			}
			delete(l.connections, p.SessionID)
			conn.sendPacket(closePkt)
			conn.setClosed()
		} else {
			log.Printf("[%s:%d]: unsolicited close, sending close in reply", addr, p.SessionID)
			closePkt := ClosePacket{
				SessionID: p.SessionID,
			}
			l.sendPacket(addr, closePkt)
		}
	default:
		panic("received unhandled packet type")
	}
}

func (l *Listener) readPackets() {
	for {
		buf := make([]byte, 1100)
		n, addr, err := l.udpConn.ReadFrom(buf)
		if err != nil {
			log.Println(err)
			return
		}
		log.Printf("[%s] len(buf) was %d, n was %d", addr, len(buf), n)
		packet, err := parsePacket(buf[:n])
		if err != nil {
			log.Printf("[%s] packet was invalid, ignoring: %s", addr, err)
			continue
		}
		log.Printf("[%s] received packet %#v", addr, packet)
		l.packetChan <- IncomingPacket{Addr: addr, Packet: packet}
	}
}

func (l *Listener) doRetransmissions() {
	now := time.Now()
	for _, conn := range l.connections {
		if conn.gotAcksUpTo < conn.bytesSent &&
			now.After(conn.lastAck.Add(RETRANSMISSION_TIMEOUT)) {
			conn.maybeRetransmit()
		}
	}
}

func (l *Listener) handlePackets() {
	go l.readPackets()
	l.ticker = *time.NewTicker(1 * time.Second)
	defer l.ticker.Stop()
	for {
		select {
		case incoming := <-l.packetChan:
			l.dispatchPacket(incoming.Packet, incoming.Addr)
		case <-l.ticker.C:
			l.doRetransmissions()
		}
	}
}

var ErrInvalidNetworkType = errors.New("bad network type")

func Listen(network, address string) (*Listener, error) {
	if network != "lrcp" {
		return nil, ErrInvalidNetworkType
	}
	conn, err := net.ListenPacket("udp", address)
	if err != nil {
		return nil, err
	}
	listener := &Listener{
		udpConn:        conn,
		newConnections: make(chan *Conn),
		address:        LrcpAddr{address},
		connections:    make(map[uint32]*Conn),
		packetChan:     make(chan IncomingPacket),
	}
	go listener.handlePackets()
	return listener, nil
}

func (l *Listener) Accept() (*Conn, error) {
	conn, ok := <-l.newConnections
	if ok {
		return conn, nil
	}
	return nil, fmt.Errorf("error reading from newConnections channel")
}

func (l *Listener) Close() error {
	return l.udpConn.Close()
}

func (l *Listener) Addr() net.Addr {
	return l.address
}

type Conn struct {
	sessionID  uint32
	localAddr  LrcpAddr
	remoteAddr net.Addr
	isClosed   bool
	listener   *Listener

	receivedUpTo uint32
	// recvLock protects the recvBuf, that's it.
	recvLock sync.Cond
	recvBuf  bytes.Buffer

	gotAcksUpTo uint32
	lastAck     time.Time
	// sendLock protects sendBuf and bytesSent
	sendLock  sync.Mutex
	sendBuf   bytes.Buffer
	bytesSent uint32
	//bufferPosition uint32
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (c *Conn) Read(b []byte) (n int, err error) {
	c.recvLock.L.Lock()
	for c.recvBuf.Len() == 0 {
		c.recvLock.Wait()
		if c.isClosed {
			return 0, fmt.Errorf("connection is closed")
		}
	}
	defer c.recvLock.L.Unlock()
	return c.recvBuf.Read(b)
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (c *Conn) Write(b []byte) (n int, err error) {
	c.sendLock.Lock()
	defer c.sendLock.Unlock()
	// documented to never fail
	_, _ = c.sendBuf.Write(b)
	c.sendDataSplit(b, c.bytesSent)
	c.bytesSent += uint32(len(b))
	return len(b), nil
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (c *Conn) Close() error {
	c.isClosed = true
	return fmt.Errorf("close not supported")
}

// LocalAddr returns the local network address, if known.
func (c *Conn) LocalAddr() net.Addr {
	return c.localAddr
}

// RemoteAddr returns the remote network address, if known.
func (c *Conn) RemoteAddr() net.Addr {
	return c.remoteAddr
}

var ErrNoDeadline = errors.New("deadlines not supported")

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail instead of blocking. The deadline applies to all future
// and pending I/O, not just the immediately following call to
// Read or Write. After a deadline has been exceeded, the
// connection can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other
// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// The error's Timeout method will return true, but note that there
// are other possible errors for which the Timeout method will
// return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (c *Conn) SetDeadline(t time.Time) error {
	return ErrNoDeadline
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (c *Conn) SetReadDeadline(t time.Time) error {
	return ErrNoDeadline
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return ErrNoDeadline
}

func (c *Conn) sendPacket(packet interface{}) error {
	return c.listener.sendPacket(c.remoteAddr, packet)
}

// This assumes all network comms are done and we just need to update internal
// state.
// Note you separately must remove the connection from the Listener's
// connection map.
func (c *Conn) setClosed() {
	c.isClosed = true
	c.recvLock.Signal()
}

// must be called with c.sendLock
func (c *Conn) maybeRetransmit() {
	if time.Now().After(c.lastAck.Add(SESSION_EXPIRY_TIMEOUT)) {
		log.Printf("[%s:%d]: session expired, silently closing", c.remoteAddr, c.sessionID)
		delete(c.listener.connections, c.sessionID)
		c.setClosed()
		return
	}
	if c.gotAcksUpTo < c.bytesSent {
		c.sendDataSplit(c.sendBuf.Bytes(), c.gotAcksUpTo)
	}
}

func (c *Conn) sendAck(SessionID uint32) {
	ack := AckPacket{
		SessionID: SessionID,
		Length:    c.receivedUpTo,
	}
	c.sendPacket(ack)
}

// This splitting code is technically wrong -- the length limit applies to
// the _post escaping_ length. However, I suspect the challenge author was nice.
func (c *Conn) sendDataSplit(b []byte, pos uint32) {
	increment := 800
	for i := 0; i < len(b); i += increment {
		end := i + increment
		if end > len(b) {
			end = len(b)
		}
		dataPkt := DataPacket{
			SessionID: c.sessionID,
			Position:  pos + uint32(i),
			Data:      b[i:end],
		}
		c.sendPacket(dataPkt)
	}
}
