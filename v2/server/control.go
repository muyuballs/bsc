package main

import (
	"fmt"
	"log"
	"net"

	"github.com/muyuballs/bsc/v2/ben"
)

func handleControlConn(conn *net.TCPConn) {
	domain, err := ben.ReadString(conn)
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}
	if len(domain) < 3 {
		log.Println("domain too short")
		conn.Close()
		return
	}
	rewrite, err := ben.ReadString(conn)
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}
	log.Println("new client [http]", domain, rewrite)
	client := NewClient(domain, rewrite, conn)
	clientMap.Append(client)
	client.StartSerivce()
}

func handleTcpTunConn(conn *net.TCPConn) {
	exportPort, err := ben.ReadInt32(conn)
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}
	if exportPort <= 0 {
		log.Println("export port must > 0")
		conn.Close()
		return
	}
	log.Println("new client[tcp]", exportPort)
	laddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("0.0.0.0:%d", exportPort))
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}
	log.Println("start listen ", laddr)
	listener, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		log.Println(err)
		conn.Close()
		return
	}
	client := NewTcpClient(int(exportPort), conn)
	client.StartSerivce()
	go func(listener *net.TCPListener, c *Client) {
		defer conn.Close()
		for {
			conn, err := listener.AcceptTCP()
			if err != nil {
				log.Println(err)
				return
			}
			c.TransferTcp(conn)
		}
	}(listener, client)
}
