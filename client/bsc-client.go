package main

import (
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
)

var serverAddr = flag.String("server", "", "bsc server addr")
var domain = flag.String("domain", "", "service public domain")
var rhost = flag.String("rhost", "", "host rewrite to")
var target = flag.String("target", "", "target service addr")
var retryCount = flag.Int("retry", 10, "connect server retry count")

func handWelcome(welcome string) {
	defer log.Println("copy done.")
	u := url.URL{Scheme: "ws", Host: *serverAddr, Path: "/welcome"}
	log.Printf("connecting to %s", u.String())
	header := make(http.Header)
	header.Set("welcome", welcome)
	c, _, err := websocket.DefaultDialer.Dial(u.String(), header)
	if err != nil {
		log.Println("dial:", err)
		return
	}
	log.Println("say welcome done.")
	defer c.Close()
	w, err := c.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return
	}
	defer w.Close()
	addr, err := net.ResolveTCPAddr("tcp", *target)
	if err != nil {
		log.Println("parse target addr:", err)
		return
	}
	log.Println("dial tcp", addr)
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		log.Println("dial target:", err)
		return
	}
	log.Println("dial target done.")
	defer conn.Close()
	mt, r, err := c.NextReader()
	if err != nil {
		log.Println("dial target:", err)
		return
	}
	var tryCount = 0
	for mt != websocket.BinaryMessage && tryCount < 5 {
		mt, r, err = c.NextReader()
		if err != nil {
			log.Println("dial target:", err)
			return
		}
		tryCount++
	}
	if mt != websocket.BinaryMessage || r == nil {
		log.Println("no binary reader gen")
		return
	}
	log.Println("start copy request")
	go io.CopyBuffer(conn, r, make([]byte, 8*1024))
	log.Println("start copy response")
	io.CopyBuffer(w, conn, make([]byte, 8*1024))
}

func connectHello(url url.URL, header http.Header) *websocket.Conn {
	c, _, err := websocket.DefaultDialer.Dial(url.String(), header)
	if err != nil {
		log.Println("dial:", err)
		return nil
	}
	log.Println("connect hello success")
	c.SetPingHandler(func(data string) error {
		log.Println("ping", data)
		c.WriteMessage(websocket.PongMessage, []byte("hello."))
		return nil
	})
	return c
}

func main() {
	log.Println("hello bsc-client")
	flag.Parse()
	if *serverAddr == "" {
		flag.PrintDefaults()
		return
	}
	if *domain == "" {
		flag.PrintDefaults()
		return
	}
	if *target == "" {
		flag.PrintDefaults()
		return
	}
	u := url.URL{Scheme: "ws", Host: *serverAddr, Path: "/hello"}
	header := make(http.Header)
	header.Set("hello", *domain)
	if *rhost != "" {
		header.Set("rhost", *rhost)
	}
	var welcomeChan = make(chan string, 5)
	go func() {
		var welcome string
		for {
			select {
			case welcome = <-welcomeChan:
				go handWelcome(welcome)
			}
		}
	}()
	retry := 0
	for {
		log.Println("try connect hello ", retry)
		c := connectHello(u, header)
		for c != nil {
			mt, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				c.Close()
				c = nil
				break
			}
			if mt == websocket.TextMessage {
				log.Printf("recv: %s", message)
				welcomeChan <- string(message)
			}
		}
		retry++
		if retry < *retryCount {
			delay := time.Second * time.Duration(retry)
			log.Println("retry connect @", time.Now().Add(delay))
			time.Sleep(delay)
		}
	}
}
