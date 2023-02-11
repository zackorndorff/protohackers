package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

//const TIMEOUT_SECONDS = 5
//const MAX_REQUESTS = 1_000_000
//const MSG_LEN = 9
//conn.SetReadDeadline(time.Now().Add(TIMEOUT_SECONDS * time.Second))
// log.Println("Handling request from", conn.RemoteAddr())

var nameRe = regexp.MustCompile(`^[A-Za-z0-9]{1,16}$`)


//func (c *Channel) Handle() {
//	for msg := range c.InChan {
//		switch v := msg.(type) {
//		case JoinMsg:

// func nameValid(name string) bool {
// 	log.Println("checking validity of", name, "with", nameRe)
// 	return nameRe.MatchString(name)
// }

func handleConnection(conn net.Conn, repo *Repository) {
	defer conn.Close()
	r := bufio.NewReader(conn)
	for {
		io.WriteString(conn, "READY\n")
		line, err := r.ReadString('\n')
		if err != nil {
			log.Printf("error reading: %s", err)
			return 
		}
		line = line[:len(line)-1]
		log.Printf("Got line %s\n", line)
		cmd, rest, found := strings.Cut(line, " ")
		cmd = strings.ToLower(cmd)
		switch cmd {
		case "list":
			if !found {
				io.WriteString(conn, "ERR usage: LIST dir\n")
				continue
			}
			list, err := repo.List(rest)
			if err != nil {
				io.WriteString(conn, fmt.Sprintf("ERR %s\n", err))
				continue
			}
			sort.Slice(list, func (i, j int) bool {
				return list[i].Name < list[j].Name
			})
			io.WriteString(conn, fmt.Sprintf("OK %d\n", len(list)))
			for _, entry := range list {
				if entry.IsOnlyDir {
					io.WriteString(conn, fmt.Sprintf("%s/ DIR\n", entry.Name))
				} else {
					io.WriteString(conn, fmt.Sprintf("%s r%d\n", entry.Name, entry.Revision))
				}
			}
		case "put":
			splits := strings.SplitN(rest, " ", 2)
			if !found || len(splits) != 2 {
				io.WriteString(conn, "ERR usage: PUT file length newline data\n")
				continue
			}
			name := splits[0]
			length_s := splits[1]
			length, err := strconv.ParseUint(length_s, 10, 32)
			if err != nil {
				io.WriteString(conn, "ERR invalid length\n")
				continue
			}
			data := make([]byte, length)
			_, err = io.ReadFull(r, data)
			if err != nil {
				log.Printf("Error reading PUT data: %s", err)
				return
			}
			rev, err := repo.Put(name, data)
			var response string
			if err != nil {
				response = fmt.Sprintf("ERR %s\n", err)
			} else {
				response = fmt.Sprintf("OK r%d\n", rev)
			}
			io.WriteString(conn, response)
		case "get":
			name, revision_s, hasRevision := strings.Cut(rest, " ")
			revision := int64(0)
			if hasRevision {
				var err error
				revision_s = strings.TrimPrefix(revision_s, "r")
				revision, err = strconv.ParseInt(revision_s, 10, 32)
				if err != nil {
					io.WriteString(conn, "ERR invalid revision\n")
					continue
				}
			}
			data, err := repo.Get(name, hasRevision, Revision(revision))
			if err != nil {
				io.WriteString(conn, fmt.Sprintf("ERR %s\n", err))
				continue
			}
			io.WriteString(conn, fmt.Sprintf("OK %d\n", len(data)))
			//io.WriteString(conn, data)
			_, err = conn.Write(data)
			if err != nil {
				log.Println("error writing GET data", err)
				return
			}
		case "help":
			io.WriteString(conn, "OK usage: HELP|GET|PUT|LIST\n")
		default:
			fmt.Printf("cmd was '%s', rest was '%s', found was %v\n", cmd, rest, found)
			io.WriteString(conn, fmt.Sprintf("ERR illegal method: %s\n", cmd))
			return
		}
	}
}

func main() {
	repository := NewRepository()

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
		go handleConnection(conn, repository)
	}
}
