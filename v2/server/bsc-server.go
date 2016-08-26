package main

import (
	"flag"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	DATA_CH_TIME_OUT = time.Second * 30
)

type CLMSG struct {
	conn    *websocket.Conn
	msgType int
	msg     []byte
}

type BSCT struct {
	w       http.ResponseWriter
	r       *http.Request
	host    string
	welcome chan *websocket.Conn
}

type Client struct {
	conn   *websocket.Conn
	domain string
	rhost  string
}

func (bsct *BSCT) Error() {
	http.Error(bsct.w, "Service Unavailable", http.StatusServiceUnavailable)
	bsct.r.Body.Close()
}

var (
	BSCTs      = make(map[string]*BSCT)
	upgrader   = websocket.Upgrader{}
	bsctsMutex = sync.Mutex{}
	clmsgChan  = make(chan CLMSG, 10)
	dcMgr      = NewChannelManager()
)

func putBsct(tag string, bsct *BSCT) {
	defer bsctsMutex.Unlock()
	bsctsMutex.Lock()
	BSCTs[tag] = bsct
}

func delBsct(tag string) {
	defer bsctsMutex.Unlock()
	bsctsMutex.Lock()
	delete(BSCTs, tag)
}

func welcomeHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("new welcome from", r.RemoteAddr)
	host := r.Header.Get("host")
	log.Println("host", host)
	if host == "" {
		log.Println("host is null")
		http.Error(w, "host is null", http.StatusBadRequest)
		return
	}
	dcMgr.HandleWelcome(host, w, r)
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	for k, v := range r.Header {
		log.Println(k, v)
	}
	host := r.Header.Get("hello")
	rhost := r.Header.Get("rhost")
	if host == "" {
		http.Error(w, "hello is null", http.StatusBadRequest)
		return
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	log.Println("new hello from", r.RemoteAddr, "@", host)
	dcMgr.RegisterClient(&Client{conn: c, domain: host, rhost: rhost})
}

func bscHandler(w http.ResponseWriter, r *http.Request) {
	channel, err := dcMgr.OpenChannel(r.Host)
	if channel == nil || err != nil {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
		log.Println(err)
		log.Println("create client data channel error")
		return
	}
	channel.Transfer(w, r)
}

func main() {
	log.Println("hello bsc-server")
	addr := flag.String("addr", "", "service port")
	flag.Parse()
	if *addr == "" {
		flag.PrintDefaults()
		return
	}
	go dcMgr.ListenCtrlMsg()
	http.HandleFunc("/hello/ws", helloHandler)
	http.HandleFunc("/welcome/ws", welcomeHandler)
	http.HandleFunc("/", bscHandler)
	http.ListenAndServe(*addr, nil)
}
