package main

import (
    "bufio"
    "errors"
    "fmt"
    "io"
    "log"
    "net"
    "strconv"
    "strings"
    "regexp"
)

type Toy struct {
    count uint32
    name string
}

func (t Toy) Response() string {
    return fmt.Sprintf("%dx %s\n", t.count, t.name)
}

var toy_re = regexp.MustCompile(`([\d]+)x (.*)`)

var ErrInvalidToy = errors.New("invalid toy string")
func getResponse(query string) (string, error) {
    toys := strings.Split(query, ",")
    biggest := Toy{}
    for _, toyStr := range toys {
        matched := toy_re.FindStringSubmatch(toyStr)
        if matched == nil {
            return "", ErrInvalidToy
        }

        num, err := strconv.ParseUint(matched[1], 10, 32)
        if err != nil {
            return "", fmt.Errorf("invalid toy count num: %w", err)
        }

        t := Toy{uint32(num), matched[2]}
        if t.count > biggest.count {
            biggest = t
        }
    }
    return biggest.Response(), nil
}

func handleConn(c net.Conn) {
    defer c.Close()
    sc := bufio.NewScanner(c)
    for sc.Scan() {
        response, err := getResponse(sc.Text())
        if err != nil {
            log.Printf("App [%s] error: %s", c.RemoteAddr(), err)
            return
        }
        log.Printf("App [%s] %s | %s", c.RemoteAddr(), sc.Text(), response)
        _, err = io.WriteString(c, response)
        if err != nil {
            log.Println(err)
            return
        }
    }
    if err := sc.Err(); err != nil {
        log.Println(err)
    }
}

func main() {
    l, err := Listen("tcp", ":1337")
    if err != nil {
        log.Fatal(err)
    }
    defer l.Close()
    log.Println("Listening.")
    for {
        conn, err := l.Accept()
        if err != nil {
            log.Println(err)
        }
        log.Println("Accepted connection from", conn.RemoteAddr())
        go handleConn(conn)
    }
}
