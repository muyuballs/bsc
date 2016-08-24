package main

import (
	"bufio"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
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

func (bsct *BSCT) Error() {
	http.Error(bsct.w, "Service Unavailable", http.StatusServiceUnavailable)
	bsct.r.Body.Close()
}

var (
	helloConns = make(map[string]*websocket.Conn)
	hostmap    = make(map[string]string)
	BSCTs      = make(map[string]*BSCT)
	upgrader   = websocket.Upgrader{}
	bsctsMutex = sync.Mutex{}
	clmsgChan  = make(chan CLMSG, 10)
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

func transfer(conn *websocket.Conn, bsct *BSCT) {
	defer log.Println("transfer done")
	defer conn.Close()
	w, err := conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		log.Println("create writer", err)
		bsct.Error()
		return
	}
	go func() {
		w.Write([]byte(fmt.Sprintf("%s %s %s\r\n", bsct.r.Method, bsct.r.RequestURI, bsct.r.Proto)))
		if bsct.host != "" {
			log.Println("rewrite host", bsct.r.Host, "-->", bsct.host)
			w.Write([]byte(fmt.Sprintf("Host: %s\r\n", bsct.host)))
		} else {
			w.Write([]byte(fmt.Sprintf("Host: %s\r\n", bsct.r.Host)))
		}
		w.Write([]byte("Connection: close\r\n"))
		bsct.r.Header.WriteSubset(w, map[string]bool{"Host": true, "Connection": true})
		w.Write([]byte("\r\n"))
		_, err := io.Copy(w, bsct.r.Body)
		if err != nil {
			log.Println("copy request body", err)
			bsct.Error()
			return
		}
		w.Write([]byte("\r\n"))
		log.Println("request body copy done")
		w.Close()
	}()
	mt, r, err := conn.NextReader()
	if err != nil {
		log.Println("gen reader", err)
		return
	}
	var tryCount = 0
	for mt != websocket.BinaryMessage && tryCount < 5 {
		mt, r, err = conn.NextReader()
		if err != nil {
			log.Println("gen reader", err)
			return
		}
		tryCount++
	}
	if mt != websocket.BinaryMessage || r == nil {
		log.Println("no binary reader gen")
		bsct.Error()
		return
	}
	log.Println("start copy response")
	reader := bufio.NewReader(r)
	resp, err := http.ReadResponse(reader, bsct.r)
	if err != nil {
		log.Println("read response", err)
		return
	}
	log.Println("read response done")
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()
	for k, v := range resp.Header {
		bsct.w.Header()[k] = v
	}
	bsct.w.WriteHeader(resp.StatusCode)
	if resp.Body != nil {
		io.CopyBuffer(bsct.w, resp.Body, make([]byte, 8*1024))
	}
}

func welcomeHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("new welcome from", r.RemoteAddr)
	target := r.Header.Get("welcome")
	log.Println("welcome", target)
	if target == "" {
		log.Println("welcome is null")
		http.Error(w, "welcome is null", http.StatusBadRequest)
		return
	}
	if bsct, ok := BSCTs[target]; ok {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusForbidden)
			bsct.Error()
			log.Println("upgrade:", err)
			return
		}
		bsct.welcome <- c
	} else {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	}
}

func heartbit(conn *websocket.Conn, domain, rhost string) {
	if rhost != "" {
		hostmap[domain] = rhost
	}
	helloConns[domain] = conn
	conn.SetPongHandler(pongHandler)
	for _ = range time.Tick(time.Minute) {
		if _, ok := helloConns[domain]; ok {
			clmsgChan <- CLMSG{conn: conn, msgType: websocket.PingMessage, msg: []byte(".hello")}
		} else {
			break
		}
	}
}

func pongHandler(data string) error {
	log.Println("pong", data)
	return nil
}

func helloHandler(w http.ResponseWriter, r *http.Request) {
	for k, v := range r.Header {
		log.Println(k, v)
	}
	host := r.Header.Get("hello")
	rhost := r.Header.Get("rhost")
	if host == "" {
		http.Error(w, "welcome is null", http.StatusBadRequest)
		return
	}
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		log.Println("upgrade:", err)
		return
	}
	log.Println("new hello from", r.RemoteAddr, "@", host)
	go heartbit(c, host, rhost)
}

func bscHandler(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if conn, ok := helloConns[host]; ok {
		target := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s@%s@%d", conn.RemoteAddr().String(), r.URL.String(), time.Now().UnixNano())))
		bsct := &BSCT{w: w, r: r, welcome: make(chan *websocket.Conn)}
		if rhost, ok := hostmap[host]; ok {
			bsct.host = rhost
		}
		putBsct(target, bsct)
		clmsgChan <- CLMSG{conn: conn, msgType: websocket.TextMessage, msg: []byte(target)}
		select {
		case wconn := <-bsct.welcome:
			delBsct(target)
			transfer(wconn, bsct)
		case <-time.After(15 * time.Second):
			delBsct(target)
			bsct.Error()
			log.Println("receive welcome time out")
		}
	} else {
		log.Println("no service for host", host)
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	}
}

func handCLMSG() {
	for {
		select {
		case clmsg := <-clmsgChan:
			log.Println("clmsg", clmsg.msgType, string(clmsg.msg), clmsg.conn.RemoteAddr())
			w, err := clmsg.conn.NextWriter(clmsg.msgType)
			if err != nil {
				log.Println("clmsg new writer", err)
				if _, ok := err.(*websocket.CloseError); ok {
					var domain = ""
					for k, v := range helloConns {
						if v == clmsg.conn {
							domain = k
							break
						}
					}
					if domain != "" {
						delete(helloConns, domain)
					}
				}
			} else {
				w.Write(clmsg.msg)
				w.Close()
			}
		}
	}
}

func main() {
	log.Println("hello bsc-server")
	addr := flag.String("addr", "", "service port")
	flag.Parse()
	if *addr == "" {
		flag.PrintDefaults()
		return
	}
	go handCLMSG()
	http.HandleFunc("/hello", helloHandler)
	http.HandleFunc("/welcome", welcomeHandler)
	http.HandleFunc("/", bscHandler)
	http.ListenAndServe(*addr, nil)
}
