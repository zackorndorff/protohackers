package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
)

type request struct {
	Method string   `json:"method"`
	Number *float64 `json:"number"`
}

type response struct {
	Method  string `json:"method"`
	IsPrime bool   `json:"prime"`
}

const MAX_REQUESTS = 5000

func handleRequest(conn net.Conn) {
	defer conn.Close()
	log.Println("Handling request from", conn.RemoteAddr())
	var err error
	requests := 0
	for s := bufio.NewScanner(conn); s.Scan() && requests < MAX_REQUESTS; requests++ {
		data := s.Text()
		log.Println("Got input", data)
		var req request
		err = json.Unmarshal([]byte(data), &req)
		if err != nil {
			log.Println("error unmarshalling", err)
			conn.Write([]byte("bad request"))
			return
		}
		if req.Method != "isPrime" {
			conn.Write([]byte("bad request"))
			return
		}

		if req.Number == nil {
			conn.Write([]byte("bad request"))
			return
		}

		isPrime := false
		if float64(int(*req.Number)) == *req.Number {
			isPrime = big.NewInt(int64(*req.Number)).ProbablyPrime(11)
		}

		res := response{
			Method:  "isPrime",
			IsPrime: isPrime,
		}

		outdata, err := json.Marshal(res)
		if err != nil {
			log.Println("Failed to marshal", err)
			return
		}
		log.Println("Reponse was", string(outdata))
		conn.Write(append(outdata, byte('\n')))
	}
	if err == io.EOF {
		log.Println("Finished serving", conn.RemoteAddr())
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
