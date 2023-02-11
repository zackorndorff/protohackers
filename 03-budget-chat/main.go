package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"strings"
)

//const TIMEOUT_SECONDS = 5
//const MAX_REQUESTS = 1_000_000
//const MSG_LEN = 9
//conn.SetReadDeadline(time.Now().Add(TIMEOUT_SECONDS * time.Second))
// log.Println("Handling request from", conn.RemoteAddr())

var nameRe = regexp.MustCompile(`^[A-Za-z0-9]{1,16}$`)

type NotifyMsg interface {
	String() string
}
type InMsg interface{}

type StringMsg struct {
	str string
}

func (s StringMsg) String() string {
	return s.str
}

type MsgMsg struct {
	Sender *Client
	Msg    string
}

func (m MsgMsg) String() string {
	return fmt.Sprintf("[%s] %s\n", m.Sender.Name, m.Msg)
}

type Client struct {
	Name   string
	Notify chan NotifyMsg
	conn   *net.Conn
}

type JoinMsg struct {
	Client *Client
}

func (j JoinMsg) String() string {
	return fmt.Sprintf("* %s has entered the room\n", j.Client.Name)
}

type LeaveMsg struct {
	Client *Client
}

func (l LeaveMsg) String() string {
	return fmt.Sprintf("* %s has left the room\n", l.Client.Name)
}

type ShutdownMsg struct{}

type Channel struct {
	InChan chan InMsg
	users  []*Client
}

func (c *Channel) Handle() {
	for msg := range c.InChan {
		switch v := msg.(type) {
		case JoinMsg:
			jmsg := msg.(JoinMsg)
			currentClients := []string{}
			for _, i := range c.users {
				i.Notify <- jmsg
				currentClients = append(currentClients, i.Name)
			}
			client := jmsg.Client

			client.Notify <- StringMsg{
				str: fmt.Sprintf("* The room contains: %s\n",
					strings.Join(currentClients, ",")),
			}

			c.users = append(c.users, client)
		case MsgMsg:
			mmsg := msg.(MsgMsg)
			for _, i := range c.users {
				if i == mmsg.Sender {
					continue
				}
				i.Notify <- mmsg
			}
		case LeaveMsg:
			lmsg := msg.(LeaveMsg)
			log.Println("processing leave of", lmsg.Client.Name)
			found := false
			i := 0
			for _, x := range c.users {
				if x != lmsg.Client {
					c.users[i] = x
					i++
				} else {
					found = true
					log.Println("Found and removing client from list")
				}
			}
			for j := i; j < len(c.users); j++ {
				c.users[i] = nil
			}
			c.users = c.users[:i]
			if found {
				close(lmsg.Client.Notify)
				(*lmsg.Client.conn).Close()
				log.Println("Notifying result")
				for _, x := range c.users {
					log.Println("  Notifying", x.Name)
					x.Notify <- lmsg
				}
			}
		case ShutdownMsg:
			for _, i := range c.users {
				close(i.Notify)
				(*i.conn).Close()
			}
			return
		default:
			log.Fatal("Unknown message type", v)
		}
	}
}

func nameValid(name string) bool {
	log.Println("checking validity of", name, "with", nameRe)
	return nameRe.MatchString(name)
}

func (c *Client) NotifyRoutine(ch *Channel) {
	for msg := range c.Notify {
		log.Println("writing out msg string", msg.String())
		_, err := io.WriteString(*c.conn, msg.String())
		if err != nil {
			log.Println("error writing msg to client", err)
			ch.InChan <- LeaveMsg{
				Client: c,
			}
			return
		}
	}
}

const GREETING = "Welcome to Budget Chat. Please enter your name.\n"

func handleConnection(conn net.Conn, channel *Channel) {
	io.WriteString(conn, GREETING)
	scanner := bufio.NewScanner(conn)
	if !scanner.Scan() {
		log.Println("Failed to read name")
		conn.Close()
		return
	}
	name := scanner.Text()
	if !nameValid(name) {
		io.WriteString(conn, "Invalid name.")
		log.Println("Got an invalid name.")
		conn.Close()
		return
	}
	log.Println("read name", name)

	notify := make(chan NotifyMsg)
	c := &Client{
		Name:   name,
		Notify: notify,
		conn:   &conn,
	}
	go c.NotifyRoutine(channel)
	joinMsg := JoinMsg{
		Client: c,
	}
	channel.InChan <- joinMsg

	for scanner.Scan() {
		channel.InChan <- MsgMsg{
			Sender: c,
			Msg:    scanner.Text(),
		}
	}

	channel.InChan <- LeaveMsg{
		Client: c,
	}
}

func newChannel() *Channel {
	return &Channel{
		InChan: make(chan InMsg),
		users:  []*Client{},
	}
}

func main() {
	channel := newChannel()
	go channel.Handle()

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
		go handleConnection(conn, channel)
	}
}
