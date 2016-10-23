package main

import (
	"bufio"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"time"

	bsc "github.com/muyuballs/bsc/v2"
	"github.com/muyuballs/bsc/v2/ben"
)

func handTcpTun() {
	if tcpPort == 0 {
		log.Println("pport must > 0")
		flag.PrintDefaults()
		return
	}
	bscAddr, err := net.ResolveTCPAddr("tcp", serverAddr)
	if err != nil {
		log.Println(err)
		return
	}
	targetAddr, err := net.ResolveTCPAddr("tcp", target)
	if err != nil {
		log.Println(err)
		return
	}
	var retryCount = 0
	for retry == -1 || retryCount <= retry {
		log.Println("dial server with:", serverAddr)
		conn, err := net.DialTCP("tcp", nil, bscAddr)
		if err == nil {
			log.Println("dial success")
			go func(conn *net.TCPConn) {
				b := bsc.Block{Type: bsc.TYPE_PING}
				for _ = range time.Tick(10 * time.Second) {
					_, err := b.WriteTo(conn)
					if err != nil {
						return
					}
				}
			}(conn)
			conn.SetKeepAlive(true)
			conn.Write([]byte{bsc.TYPE_TCP})
			ben.WriteInt32(conn, int32(tcpPort))
			handConn(TCP, conn, targetAddr)
		} else {
			log.Println(err)
		}
		retryCount++
		atTime := time.Now().Add(10 * time.Second)
		log.Println("retry connect server @", atTime)
		time.Sleep(10 * time.Second)
	}
}

func openTcpChannel(bscConn *net.TCPConn, tag int32, targetAddr *net.TCPAddr) {
	targetConn, err := net.DialTCP("tcp", nil, targetAddr)
	if err != nil {
		closeTag(bscConn, tag, err)
		return
	}
	targets[tag] = targetConn
	go func(tag int32) {
		io.Copy(bsc.NewBlockWriter(bscConn, tag), bufio.NewReader(targetConn))
		closeTag(bscConn, tag, errors.New("copy done."))
	}(tag)
}
