package main

import (
	"bufio"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	bsc "github.com/muyuballs/bsc/v2"
	"github.com/muyuballs/bsc/v2/ben"
)

func handHttpTun() {
	if domain == "" {
		log.Println("domain must not null")
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
			conn.SetKeepAlive(true)
			conn.Write([]byte{bsc.TYPE_HTTP})
			ben.WriteString(conn, domain)
			ben.WriteString(conn, rhost)
			handConn(HTTP, conn, targetAddr)
		} else {
			log.Println(err)
		}
		retryCount++
		atTime := time.Now().Add(10 * time.Second)
		log.Println("retry connect server @", atTime)
		time.Sleep(10 * time.Second)
	}
}

func dialHttpTarget(taddr *net.TCPAddr) (conn io.ReadWriteCloser, err error) {
	log.Println("dial taddr", taddr)
	if taddr.Port == 443 || isTls {
		dConn, err := tls.Dial("tcp", taddr.String(), &tls.Config{InsecureSkipVerify: true})
		if err != nil {
			log.Println("dial taddr:", err)
			return nil, err
		}
		log.Println("dial taddr with tls done.")
		return dConn, nil
	}
	dConn, err := net.DialTCP("tcp", nil, taddr)
	if err != nil {
		log.Println("dial taddr:", err)
		return
	}
	log.Println("dial taddr done.")
	return dConn, nil
}

func openHttpChannel(bscConn *net.TCPConn, tag int32, targetAddr *net.TCPAddr) {
	startTime := time.Now()
	targetConn, err := dialHttpTarget(targetAddr)
	if err != nil {
		closeTag(bscConn, tag, err)
		return
	}
	targets[tag] = targetConn
	go func(tag int32, dialCost time.Duration) {
		r := bufio.NewReader(targetConn)
		l, m, err := r.ReadLine()
		if err != nil {
			closeTag(bscConn, tag, err)
			return
		}
		if m {
			closeTag(bscConn, tag, errors.New("tooo lang status line."))
			return
		}
		w := bsc.NewBlockWriter(bscConn, tag)
		w.Write(l)
		w.Write([]byte{'\r', '\n'})
		fmt.Fprintf(w, "X-BSC-DIAL:%s\r\n", dialCost.String())
		_, err = io.CopyBuffer(w, r, make([]byte, bsc.DEF_BUF))
		if err != nil {
			log.Println("copy", tag, err)
			return
		}
		closeTag(bscConn, tag, errors.New("copy done."))
	}(tag, time.Since(startTime))
}
