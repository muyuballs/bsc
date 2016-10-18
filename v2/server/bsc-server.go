package main

import (
	"flag"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	bsc "github.com/muyuballs/bsc/v2"
)

var (
	clientMap = &ClientMap{
		locker:  sync.Mutex{},
		clients: make(map[string]*Client),
	}
)

func bscHandler(w http.ResponseWriter, r *http.Request) {
	if client, ok := clientMap.clients[r.Host]; ok {
		client.Transfer(w, r)
	} else {
		bsc.ServiceUnavailable(w, r)
		log.Println("no client service for", r.Host)
	}
}

func handleControlOrDataConnection(conn *net.TCPConn) {
	log.Println("new client from", conn.RemoteAddr().String())
	c_type, err := bsc.ReadByte(conn)
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}
	switch c_type {
	case bsc.TYPE_INIT:
		handleControlConn(conn)
	default:
		log.Println("not support c_type:", c_type)
		conn.Close()
	}
}

func pingTask() {
	for _ = range time.Tick(10 * time.Second) {
		errClients := make([]string, 0)
		for domain, client := range clientMap.clients {
			if err := client.SendPingMessage(); err != nil {
				log.Println(err)
				client.Close()
				errClients = append(errClients, domain)
			}
		}
		clientMap.RemoveAll(errClients)
	}
}

func listenControlAndDataPort(addr string) (err error) {
	laddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return
	}
	listener, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		return
	}
	go func() {
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				panic(err)
				return
			}
			go handleControlOrDataConnection(conn)
		}
	}()
	return
}

func main() {
	log.Println("hello bsc-server")
	httpAddr := flag.String("http", "", "http listen address")
	tcp := flag.String("tcp", "", "data & control listen address")
	flag.Parse()
	if *httpAddr == "" {
		flag.PrintDefaults()
		return
	}
	if *tcp == "" {
		flag.PrintDefaults()
		return
	}
	err := listenControlAndDataPort(*tcp)
	if err != nil {
		log.Println(err)
		return
	}
	go pingTask()
	http.HandleFunc("/", bscHandler)
	log.Println(http.ListenAndServe(*httpAddr, nil))
}
