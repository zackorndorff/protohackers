package main

import (
    "fmt"
    "io"
    "log"
    "net"
    "time"
)

type Listener struct {
    net.Listener
}

func Listen(network, address string) (net.Listener, error) {
    tcpL, err := net.Listen(network, address)
    if err != nil {
        return nil, err
    }
    return &Listener {
        Listener: tcpL,
    }, nil
}

// Accept waits for and returns the next connection to the listener.
func (l *Listener) Accept() (net.Conn, error) {
    conn, err := l.Listener.Accept()
    if err != nil {
        return nil, err
    }
    return &Conn {
        Conn: conn,
    }, nil
}

// Close closes the listener.
// Any blocked Accept operations will be unblocked and return errors.
func (l *Listener) Close() error{
    return l.Listener.Close()
}

// Addr returns the listener's network address.
func (l *Listener) Addr() net.Addr{
    return l.Listener.Addr()
}

type Conn struct {
    net.Conn
    ciphers []Cipher
    readpos, writepos uint64
}

func (c *Conn) EnsureHandshake() error {
    if len(c.ciphers) == 0 {
        ciphers, err := parseHandshake(c.Conn)
        if err != nil {
            log.Println("Error handshaking with client", c.RemoteAddr(), err)
            c.Close()
            return err
        }
        c.ciphers = ciphers
        log.Println("Client handshake success", c.RemoteAddr(), c.ciphers)
    }
    return nil
}

func parseHandshake(r io.Reader) ([]Cipher, error) {
    ciphers := make([]Cipher, 0)
    buf := make([]byte, 1)
    previous := byte(0)
    for {
        _, err := r.Read(buf)
        if err != nil {
            return nil, err
        }
        if previous != 0 {
            ciphers = append(ciphers, GetCipher(previous, buf[0]))
            previous = 0
        } else {
            if buf[0] >= MaxCipher {
                return nil, fmt.Errorf("Unsupported cipher %d", buf[0])
            } else if buf[0] == End {
                break
            }
            if SecondByteNeeded[buf[0]] {
                previous = buf[0]
            } else {
                previous = 0
                ciphers = append(ciphers, GetCipher(buf[0], 0))
            }
        }
    }
    if len(ciphers) == 0 {
        return nil, fmt.Errorf("no ciphers selected")
    }
    if isNoOpCipher(ciphers) {
        return nil, fmt.Errorf("no op cipher, bad")
    }
    return ciphers, nil
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
func (c *Conn) Read (b []byte) (n int, err error) {
    err = c.EnsureHandshake()
    if err != nil {
        return 0, err
    }
    n, err = c.Conn.Read(b)
    Crypt(c.ciphers, b[0:n], c.readpos, true)
    c.readpos += uint64(n)
    return n, err
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
func (c *Conn) Write(b []byte) (n int, err error) {
    err = c.EnsureHandshake()
    if err != nil {
        return 0, err
    }
    tmp := make([]byte, len(b))
    copy(tmp, b)
    Crypt(c.ciphers, tmp, c.writepos, false)
    c.writepos += uint64(len(tmp))
    return c.Conn.Write(tmp)
}

// Close closes the connection.
// Any blocked Read or Write operations will be unblocked and return errors.
func (c *Conn) Close() error {
    return c.Conn.Close()
}

// LocalAddr returns the local network address, if known.
func (c *Conn) LocalAddr() net.Addr {
    return c.Conn.LocalAddr()
}

// RemoteAddr returns the remote network address, if known.
func (c *Conn) RemoteAddr() net.Addr {
    return c.Conn.RemoteAddr()
}

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
    return c.Conn.SetDeadline(t)
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (c *Conn) SetReadDeadline(t time.Time) error {
    return c.Conn.SetReadDeadline(t)
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (c *Conn) SetWriteDeadline(t time.Time) error {
    return c.Conn.SetWriteDeadline(t)
}
