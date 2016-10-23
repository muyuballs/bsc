package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	CH_C_OUT        = make(chan (int), 100)
	CH_C_IN         = make(chan (int), 100)
	clientIn  int64 = 0
	clientOut int64 = 0
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

func trafficTask() {
	for {
		select {
		case v := <-CH_C_IN:
			clientIn += int64(v)
		case v := <-CH_C_OUT:
			clientOut += int64(v)
		case _ = <-time.Tick(time.Second):
			continue
		}
	}
}

func infoTask() {
	var _ci int64 = 0
	var _co int64 = 0
	for _ = range time.Tick(time.Second) {
		cis := clientIn - _ci
		cos := clientOut - _co
		_ci = clientIn
		_co = clientOut
		log.Printf("Client >> In:%s Out:%s \n", formatSpeed(cis), formatSpeed(cos))
	}
}

func formatSpeed(val int64) string {
	if val>>30 > 0 {
		return fmt.Sprintf("%vGB", val*1.0/(1<<30))
	}
	if val>>20 > 0 {
		return fmt.Sprintf("%vMB", val*1.0/(1<<20))
	}
	if val>>10 > 0 {
		return fmt.Sprintf("%vKB", val*1.0/(1<<10))
	}
	return fmt.Sprintf("%dB", val)
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
	go trafficTask()
	go infoTask()
	http.HandleFunc("/", bscHandler)
	log.Println(http.ListenAndServe(Config.Http, nil))
}
