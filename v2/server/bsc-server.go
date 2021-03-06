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

type serverConfig struct {
	HTTP      string
	Control   string
	TCPEnable bool
}

var (
	conf      = ""
	config    = &serverConfig{}
	clientMap = &ClientMap{
		locker:  sync.Mutex{},
		clients: make(map[string]*Client),
	}

//	CH_C_OUT        = make(chan (int), 100)
//	CH_C_IN         = make(chan (int), 100)
//	clientIn  int64 = 0
//	clientOut int64 = 0
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
	cType, err := bsc.ReadByte(conn)
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}
	switch cType {
	case bsc.TYPE_HTTP:
		handleHTTPControlConn(conn)
	case bsc.TYPE_TCP:
		if config.TCPEnable {
			handleTCPTunConn(conn)
		} else {
			log.Println("tcp tun is not enabled")
			conn.Close()
		}
	default:
		log.Println("not support type:", cType)
		conn.Close()
	}
}

func pingTask() {
	for _ = range time.Tick(30 * time.Second) {
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

//func trafficTask() {
//	for {
//		select {
//		case v := <-CH_C_IN:
//			clientIn += int64(v)
//		case v := <-CH_C_OUT:
//			clientOut += int64(v)
//		case _ = <-time.Tick(time.Second):
//			continue
//		}
//	}
//}

//func infoTask() {
//	var _ci int64 = 0
//	var _co int64 = 0
//	for _ = range time.Tick(time.Second) {
//		cis := clientIn - _ci
//		cos := clientOut - _co
//		_ci = clientIn
//		_co = clientOut
//		log.Printf("Client >> In:%s Out:%s \n", bsc.FormatSize(cis), bsc.FormatSize(cos))
//	}
//}

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
			}
			go handleControlOrDataConnection(conn)
		}
	}()
	return
}

func main() {
	log.Println("hello bsc-server")
	flag.StringVar(&config.HTTP, "http", "", "http listen address")
	flag.StringVar(&config.Control, "ctrl", "", "controller listen address")
	flag.BoolVar(&config.TCPEnable, "tcp", false, "tcp tun enable ,default is false")
	flag.StringVar(&conf, "conf", "", "conf file path")
	flag.Parse()
	if conf != "" {
		data, err := ioutil.ReadFile(conf)
		if err != nil {
			log.Println(err)
			return
		}
		err = json.Unmarshal(data, config)
		if err != nil {
			log.Println(err)
			return
		}
	}
	if config.HTTP == "" {
		log.Println("http must not null")
		flag.PrintDefaults()
		return
	}
	if config.Control == "" {
		log.Println("ctrl must not null")
		flag.PrintDefaults()
		return
	}
	err := listenControlPort(config.Control)
	if err != nil {
		log.Println(err)
		return
	}
	go pingTask()
	//	go trafficTask()
	//	go infoTask()
	http.HandleFunc("/", bscHandler)
	log.Println(http.ListenAndServe(config.HTTP, nil))
}
