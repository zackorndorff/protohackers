package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"time"
)

type RequestType uint8

type InsertRequest struct {
	Timestamp int32
	Price     int32
}

type QueryRequest struct {
	Mintime int32
	Maxtime int32
}

type Price struct {
	Timestamp time.Time
	Price     int32
}

type Connection struct {
	Prices []Price
}

func (r *InsertRequest) ToPrice() Price {
	return Price{
		Timestamp: time.UnixMilli(int64(r.Timestamp)),
		Price:     r.Price,
	}
}

func BeforeEqual(t1, t2 *time.Time) bool {
	return t1.Before(*t2) || t1.Equal(*t2)
}

func (c *Connection) Insert(p Price) {
	c.Prices = append(c.Prices, p)
}

// do a query
func (c *Connection) Query(mintime, maxtime time.Time) int32 {
	var total int64
	var count int64
	for _, price := range c.Prices {
		if BeforeEqual(&mintime, &price.Timestamp) && BeforeEqual(&price.Timestamp, &maxtime) {
			total += int64(price.Price)
			count++
		}
	}
	if count == 0 {
		return 0
	}
	return int32(total / count)
}

const TIMEOUT_SECONDS = 5
const MAX_REQUESTS = 1_000_000
const MSG_LEN = 9

func handleRequest(conn net.Conn) {
	defer conn.Close()
	log.Println("Handling request from", conn.RemoteAddr())
	var err error
	requests := 0
	cState := Connection{}

	data := make([]byte, MSG_LEN)

	for ; requests < MAX_REQUESTS && err == nil; requests++ {
		conn.SetReadDeadline(time.Now().Add(TIMEOUT_SECONDS * time.Second))
		_, err = io.ReadFull(conn, data)
		if err != nil {
			break
		}
		pktbuf := bytes.NewReader(data)
		var msgtype byte
		msgtype, err = pktbuf.ReadByte()
		if err != nil {
			break
		}
		switch msgtype {
		case byte('Q'):
			var req QueryRequest
			err = binary.Read(pktbuf, binary.BigEndian, &req)
			if err != nil {
				break
			}
			result := cState.Query(time.UnixMilli(int64(req.Mintime)), time.UnixMilli(int64(req.Maxtime)))
			binary.Write(conn, binary.BigEndian, &result)
		case byte('I'):
			var req InsertRequest
			err = binary.Read(pktbuf, binary.BigEndian, &req)
			if err != nil {
				break
			}
			cState.Insert(req.ToPrice())
		default:
			err = fmt.Errorf("invalid packet type")
		}
	}

	if err == io.EOF {
		log.Printf("Finished serving %s (%d requests)\n", conn.RemoteAddr(), requests)
	} else if err == nil && requests == MAX_REQUESTS {
		log.Println("Too many requests from", conn.RemoteAddr())
	} else {
		log.Println("Error reading", err, conn.RemoteAddr())
	}
}

func main() {
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
		go handleRequest(conn)
	}
}
