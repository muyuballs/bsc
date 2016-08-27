package main

import (
	"bufio"
	"flag"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	bsc "github.com/muyuballs/bsc/v2"
)

var serverAddr = flag.String("server", "", "bsc server addr")
var domain = flag.String("domain", "", "service public domain")
var rhost = flag.String("rhost", "", "host rewrite to")
var target = flag.String("target", "", "target service addr")
var daddr = flag.String("daddr", "", "data transfer addr")
var (
	CWriter = make(map[byte]io.Writer)
)

func openConn(taddr *net.TCPAddr) (conn *net.TCPConn, err error) {
	log.Println("dial taddr", taddr)
	dConn, err := net.DialTCP("tcp", nil, taddr)
	if err != nil {
		log.Println("dial taddr:", err)
		return
	}
	log.Println("dial taddr done.")
	return dConn, nil
}
func handWelcome(welcome string, daddr, taddr *net.TCPAddr) {
	defer log.Println("copy done.")
	log.Println("dial daddr", daddr)
	dConn, err := net.DialTCP("tcp", nil, daddr)
	if err != nil {
		log.Println("dial daddr:", err)
		return
	}
	bufw := bufio.NewWriter(dConn)
	bufw.WriteString(welcome)
	bufw.WriteByte('\n')
	bufw.Flush()
	log.Println("dial daddr done.")
	defer dConn.Close()
	CWriter = make(map[byte]io.Writer)
	blockReader := bsc.BlockReader{Reader: dConn}
	for {
		block, err := blockReader.Read()
		if err != nil {
			log.Println("read data channel ", err)
			break
		}
		if block.Type == bsc.BL_TYPE_CLOSE {
			log.Println("close channel", block.Tag)
			delete(CWriter, block.Tag)
			continue
		}
		if block.Type == bsc.BL_TYPE_OPEN {
			log.Println("open channel", block.Tag)
			tConn, err := openConn(taddr)
			if err != nil {
				continue
			}
			go io.Copy(bsc.NewBlockWriter(dConn, block.Tag), tConn)
			CWriter[block.Tag] = tConn
		}
		if writer, ok := CWriter[block.Tag]; ok {
			n, err := writer.Write(block.Data)
			if err != nil || n < len(block.Data) {
				if err != nil {
					log.Println(err)
				} else {
					log.Println(io.ErrShortWrite)
				}
				delete(CWriter, block.Tag)
			}
		}
	}

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

//
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
	if *daddr == "" {
		flag.PrintDefaults()
		return
	}
	u := url.URL{Scheme: "ws", Host: *serverAddr, Path: "/hello/ws"}
	header := make(http.Header)
	header.Set("hello", *domain)
	if *rhost != "" {
		header.Set("rhost", *rhost)
	}
	taddr, err := net.ResolveTCPAddr("tcp", *target)
	if err != nil {
		log.Println(err)
		return
	}
	daddr, err := net.ResolveTCPAddr("tcp", *daddr)
	if err != nil {
		log.Println(err)
		return
	}
	var welcomeChan = make(chan string, 5)
	go func() {
		var welcome string
		for {
			select {
			case welcome = <-welcomeChan:
				log.Println(welcome)
				go handWelcome(welcome, daddr, taddr)
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
		if retry < 10 {
			delay := time.Second * time.Duration(retry)
			log.Println("retry connect @", time.Now().Add(delay))
			time.Sleep(delay)
		} else {
			break
		}
	}
}
