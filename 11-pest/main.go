package main

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"time"
	"z10f.com/golang/protohackers/11/protocol"
)

func validVisit(msg *protocol.MsgSiteVisit) bool {
	seen := make(map[string]uint32)
	for _, obs := range msg.Populations {
		prev, ok := seen[obs.Species]
		if ok && prev != obs.Count {
			return false
		}
		seen[obs.Species] = obs.Count
	}
	return true
}

func sendMsg(conn net.Conn, msg interface{}) error {
	m := protocol.NewUnmarshallerForTesting()
	bufw := bufio.NewWriter(conn)
	log.Printf("Sending message %#v", msg)
	err := m.MarshalMessage(bufw, msg)
	if err != nil {
		log.Println("error sending message", err, msg)
		return err
	}
	err = bufw.Flush()
	if err != nil {
		log.Println("error flushing data", err)
		return err
	}
	log.Println("flushed")
	return nil
}

func errorOut(conn net.Conn, reason string) {
	sendMsg(conn, &protocol.MsgError{Message: reason})
}

func handleConnection(conn net.Conn, auth *Authority) {
	conn.SetReadDeadline(time.Now().Add(30 * time.Second))
	log.Println("\n\nHandling connection from", conn.RemoteAddr())
	shouldClose := true
	defer func() {
		go func() {
			time.Sleep(1 * time.Second)
			if shouldClose {
				conn.Close()
			}
		}()
	}()
	sendMsg(conn, &protocol.MsgHello{
		Protocol: "pestcontrol",
		Version:  1,
	})
	u := protocol.NewUnmarshallerForServerEnd()
	gotHello := false
	for {
		msg, err := u.UnmarshalMessage(conn)
		if errors.Is(err, protocol.ErrInvalidMsgType) {
			errorOut(conn, "invalid message type")
			return
		} else if err != nil {
			log.Println("error reading:", err)
			errorOut(conn, fmt.Sprintf("error reading: %s", err))
			shouldClose = false
			return
		}
		log.Printf("Got message %#v", msg)
		switch msg := msg.(type) {
		case *protocol.MsgHello:
			if gotHello {
				errorOut(conn, "already got hello")
				return
			}
			if msg.Protocol != "pestcontrol" {
				errorOut(conn, "bad hello protocol")
				return
			}
			if msg.Version != 1 {
				errorOut(conn, "bad hello version")
				return
			}
			gotHello = true
		case *protocol.MsgSiteVisit:
			if !gotHello {
				errorOut(conn, "didn't get hello")
				return
			}
			if !validVisit(msg) {
				log.Println("got invalid observation")
				errorOut(conn, "invalid observation")
				return
			}
			auth.HandleVisit(msg)
		default:
			errorOut(conn, "bad message type")
			return
		}
	}
}

func main() {
	auth := NewAuthority()

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
		// client := &Client{
		// 	conn:            conn,
		// 	Core:            state,
		// 	HeartbeatConfig: make(chan DeciSecond),
		// 	Message:         make(chan interface{}),
		// }
		go handleConnection(conn, auth) //, state)
	}
}
