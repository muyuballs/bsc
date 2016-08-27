package main

import (
	"bufio"
	"flag"
	"log"
	"net"
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

func helloHandler(w http.ResponseWriter, r *http.Request) {
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

func handleWelcome(conn *net.TCPConn) {
	bufr := bufio.NewReader(conn)
	dat, _, err := bufr.ReadLine()
	if err != nil {
		conn.Close()
		log.Println(err)
	}
	log.Println("wel", string(dat))
	dcMgr.HandleWelcome(string(dat), conn)
}

func listenDataConnect(addr string) (err error) {
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
				log.Println(err)
				return
			}
			log.Println("new data conn", conn.RemoteAddr())
			go handleWelcome(conn)
		}
	}()
	return
}

func main() {
	log.Println("hello bsc-server")
	addr := flag.String("addr", "", "service port")
	daddr := flag.String("daddr", "", "data service addr")
	flag.Parse()
	if *addr == "" {
		flag.PrintDefaults()
		return
	}
	if *daddr == "" {
		flag.PrintDefaults()
		return
	}
	go dcMgr.ListenCtrlMsg()
	err := listenDataConnect(*daddr)
	if err != nil {
		log.Println(err)
		return
	}
	http.HandleFunc("/hello/ws", helloHandler)
	http.HandleFunc("/", bscHandler)
	log.Println(http.ListenAndServe(*addr, nil))
}
