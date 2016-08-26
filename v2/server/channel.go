package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	bsc "github.com/muyuballs/bsc/v2"
)

const DC_MAX_REF = 10

type DomainChannel struct {
	ref     int
	Rhost   string
	Writer  bsc.ChunckWriter
	Reader  io.Reader
	CWriter map[byte]io.Writer
}

func (dc *DomainChannel) Start(reader bsc.ChunckReader) {

}

func (dc *DomainChannel) NewDataChannel() *DataChannel {
	if dc.ref >= DC_MAX_REF {
		return nil
	}
	dc.ref++
	r, w := io.Pipe()
	channel := &DataChannel{
		Tag:    byte(dc.ref),
		Rhost:  dc.Rhost,
		Writer: dc.Writer,
		Reader: r,
	}
	dc.CWriter[channel.Tag] = w
	return channel
}

func CreateDataChannel(conn *websocket.Conn) (dc *DataChannel, err error) {
	var r_mt int
	var br io.Reader
	c := 0
	for {
		mt, _br, err := conn.NextReader()
		if err != nil {
			return nil, err
		}
		br = _br
		r_mt = mt
		c++
		if mt == websocket.BinaryMessage || c >= 5 {
			break
		}
	}
	if r_mt != websocket.BinaryMessage {
		return nil, errors.New("no binary reader found")
	}
	bw, err := conn.NextWriter(websocket.BinaryMessage)
	if err != nil {
		return
	}
	r, w := io.Pipe()
	dc = &DataChannel{
		Writer:  bsc.ChunckWriter{Writer: bw},
		Reader:  r,
		Tag:     1,
		CWriter: make(map[byte]io.Writer),
	}
	dc.CWriter[dc.Tag] = w
	go dc.Start(bsc.ChunckReader{Reader: br})
	return
}

type CtrlMsg struct {
	client  *Client
	msgType int
	msg     []byte
}

const WelComeTimeOut = 15 * time.Second

type ChannelManager struct {
	dataChannel map[string][]*DomainChannel
	clients     map[string]*Client
	clmsgChan   chan CtrlMsg
	welChan     map[string]chan *websocket.Conn
}

func NewChannelManager() *ChannelManager {
	return &ChannelManager{
		dataChannel: make(map[string][]*DomainChannel),
		clients:     make(map[string]*Client),
		clmsgChan:   make(chan CtrlMsg, 10),
		welChan:     make(map[string]chan *websocket.Conn)}
}

func (cm *ChannelManager) GetClient(domain string) *Client {
	if cl, ok := cm.clients[domain]; ok {
		return cl
	}
	return nil
}

func (cm *ChannelManager) HandleWelcome(host string, w http.ResponseWriter, r *http.Request) {
	if ch, ok := cm.welChan[host]; ok {
		c, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println("upgrade:", err)
			return
		}
		ch <- c
	} else {
		http.Error(w, "Service Unavailable", http.StatusServiceUnavailable)
	}
}

func (cm *ChannelManager) OpenChannel(domain string) (dc *DataChannel, err error) {
	if cl, ok := cm.clients[domain]; ok {
		if dcs, ok := cm.dataChannel[cl.domain]; ok {
			for _, dc := range dcs {
				p := dc.Copy()
				if p != nil {
					return p, nil
				}
			}
		}
		cm.clmsgChan <- CtrlMsg{client: cl, msgType: websocket.TextMessage, msg: []byte(domain)}
		cm.welChan[domain] = make(chan *websocket.Conn)
		select {
		case conn := <-cm.welChan[domain]:
			delete(cm.welChan, domain)
			p, err := CreateDataChannel(conn)
			if err != nil {
				return nil, err
			}
			cm.dataChannel[cl.domain] = append(cm.dataChannel[cl.domain], p)
			return p, nil
		case <-time.Tick(WelComeTimeOut):
			delete(cm.welChan, domain)
			return nil, errors.New("create channel time out")
		}
	}
	return nil, errors.New("Service Unavailable")
}

func (cm *ChannelManager) RegisterClient(client *Client) {
	cm.clients[client.domain] = client
	cm.dataChannel[client.domain] = make([]*DomainChannel, 0)
	go heartbit(client.domain, cm)
}

func (cm *ChannelManager) CloseClient(client *Client) {
	delete(cm.clients, client.domain)
	if dcs, ok := cm.dataChannel[client.domain]; ok {
		for _, dc := range dcs {
			dc.Close()
		}
	}
	delete(cm.dataChannel, client.domain)
}

func (cm *ChannelManager) ListenCtrlMsg() {
	for {
		select {
		case clmsg := <-cm.clmsgChan:
			client := clmsg.client
			log.Println("clmsg", clmsg.msgType, string(clmsg.msg), client.conn.RemoteAddr())
			w, err := client.conn.NextWriter(clmsg.msgType)
			if err != nil {
				log.Println("clmsg new writer", err)
				if _, ok := err.(*websocket.CloseError); ok {
					cm.CloseClient(client)
				}
			} else {
				w.Write(clmsg.msg)
				w.Close()
			}
		}
	}
}

func heartbit(domain string, cm *ChannelManager) {
	for _ = range time.Tick(time.Minute) {
		client := cm.GetClient(domain)
		if client != nil {
			cm.clmsgChan <- CtrlMsg{client: client, msgType: websocket.PingMessage, msg: []byte(".hello")}
		} else {
			break
		}
	}
}
