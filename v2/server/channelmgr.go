package main

import (
	"encoding/base64"
	"errors"
	"log"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type CtrlMsg struct {
	client  *Client
	msgType int
	msg     []byte
}

const WelComeTimeOut = 15 * time.Second

type ChannelManager struct {
	domainChannel map[string][]*DomainChannel
	clients       map[string]*Client
	ctrlMsgChan   chan CtrlMsg
	welcomeChan   map[string]chan *net.TCPConn
}

func NewChannelManager() *ChannelManager {
	return &ChannelManager{
		domainChannel: make(map[string][]*DomainChannel),
		clients:       make(map[string]*Client),
		ctrlMsgChan:   make(chan CtrlMsg, 10),
		welcomeChan:   make(map[string]chan *net.TCPConn)}
}

func (cm *ChannelManager) GetClient(domain string) *Client {
	if cl, ok := cm.clients[domain]; ok {
		return cl
	}
	return nil
}

func (cm *ChannelManager) HandleWelcome(wel string, conn *net.TCPConn) {
	if ch, ok := cm.welcomeChan[wel]; ok {
		ch <- conn
	} else {
		conn.Close()
	}
}

func (cm *ChannelManager) OpenChannel(domain string) (dc *DataChannel, err error) {
	log.Println("open channel")
	if cl, ok := cm.clients[domain]; ok {
		if dcs, ok := cm.domainChannel[cl.domain]; ok {
			for _, dc := range dcs {
				p := dc.NewDataChannel()
				if p != nil {
					return p, nil
				}
			}
		}
		welStr := base64.StdEncoding.EncodeToString([]byte(domain))
		defer delete(cm.welcomeChan, welStr)
		cm.welcomeChan[welStr] = make(chan *net.TCPConn)
		cm.ctrlMsgChan <- CtrlMsg{client: cl, msgType: websocket.TextMessage, msg: []byte(welStr)}
		select {
		case conn := <-cm.welcomeChan[welStr]:
			dc, err := CreateDomainChannel(conn)
			if err != nil {
				return nil, err
			}
			dc.Rhost = cl.rhost
			cm.domainChannel[domain] = append(cm.domainChannel[domain], dc)
			return dc.NewDataChannel(), nil
		case <-time.Tick(WelComeTimeOut):
			return nil, errors.New("create channel time out")
		}
	}
	return nil, errors.New("Service Unavailable")
}

func (cm *ChannelManager) RegisterClient(client *Client) {
	cm.clients[client.domain] = client
	cm.domainChannel[client.domain] = make([]*DomainChannel, 0)
	go heartbit(client.domain, cm)
}

func (cm *ChannelManager) CloseClient(client *Client) {
	delete(cm.clients, client.domain)
	if dcs, ok := cm.domainChannel[client.domain]; ok {
		for _, dc := range dcs {
			dc.Close()
		}
	}
	delete(cm.domainChannel, client.domain)
}

func (cm *ChannelManager) ListenCtrlMsg() {
	for {
		select {
		case clmsg := <-cm.ctrlMsgChan:
			client := clmsg.client
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
			cm.ctrlMsgChan <- CtrlMsg{client: client, msgType: websocket.PingMessage, msg: []byte(".hello")}
		} else {
			break
		}
	}
}
