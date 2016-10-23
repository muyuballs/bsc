package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	bsc "github.com/muyuballs/bsc/v2"
)

type ServerConfig struct {
	Http      string
	Control   string
	TcpEnable bool
}

var (
	Conf      = ""
	Config    = &ServerConfig{}
	clientMap = &ClientMap{
		locker:  sync.Mutex{},
		clients: make(map[string]*Client),
	}
)

func bscHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	if client, ok := clientMap.clients[r.Host]; ok {
		client.Transfer(w, r, startTime)
	} else {
		bsc.ServiceUnavailable(w, r)
		log.Println("no client service for", r.Host)
	}
}

func handleControlOrDataConnection(conn *net.TCPConn) {
	c_type, err := bsc.ReadByte(conn)
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}
	switch c_type {
	case bsc.TYPE_HTTP:
		handleControlConn(conn)
	case bsc.TYPE_TCP:
		if Config.TcpEnable {
			handleTcpTunConn(conn)
		} else {
			log.Println("tcp tun is not enabled")
			conn.Close()
		}
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

func listenControlPort(addr string) (err error) {
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
	flag.StringVar(&Config.Http, "http", "", "http listen address")
	flag.StringVar(&Config.Control, "ctrl", "", "controller listen address")
	flag.BoolVar(&Config.TcpEnable, "tcp", false, "tcp tun enable ,default is false")
	flag.StringVar(&Conf, "conf", "", "conf file path")
	flag.Parse()
	if Conf != "" {
		data, err := ioutil.ReadFile(Conf)
		if err != nil {
			log.Println(err)
			return
		}
		err = json.Unmarshal(data, Config)
		if err != nil {
			log.Println(err)
			return
		}
	}
	if Config.Http == "" {
		log.Println("http must not null")
		flag.PrintDefaults()
		return
	}
	if Config.Control == "" {
		log.Println("ctrl must not null")
		flag.PrintDefaults()
		return
	}
	err := listenControlPort(Config.Control)
	if err != nil {
		log.Println(err)
		return
	}
	go pingTask()
	http.HandleFunc("/", bscHandler)
	log.Println(http.ListenAndServe(Config.Http, nil))
}
