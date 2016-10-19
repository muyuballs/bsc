package main

import (
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
	log.Println("new client", domain, rewrite)
	client := NewClient(domain, rewrite, conn)
	clientMap.Append(client)
	client.StartSerivce()
}
