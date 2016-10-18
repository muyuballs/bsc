package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"time"

	bsc "github.com/muyuballs/bsc/v2"
	"github.com/muyuballs/bsc/v2/ben"
)

var serverAddr = flag.String("server", "", "bsc server addr")
var domain = flag.String("domain", "", "service public domain")
var rhost = flag.String("rhost", "", "host rewrite to")
var target = flag.String("target", "", "target service addr")
var retry = flag.Int("retry count", -1, "retry count default -1 for ever")
var isTls = flag.Bool("tls", false, "is https")

func dialTarget(taddr *net.TCPAddr) (conn io.ReadWriteCloser, err error) {
	log.Println("dial taddr", taddr)
	if taddr.Port == 443 || *isTls {
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

func closeTag(conn *net.TCPConn, tag int32, err error) {
	log.Println("close channel", tag, err)
	b := bsc.Block{Tag: tag, Type: bsc.TYPE_CLOSE}
	b.WriteTo(conn)
}

func pang(conn *net.TCPConn) {
	b := bsc.Block{Type: bsc.TYPE_PANG}
	b.WriteTo(conn)
}

func handConn(serverConn *net.TCPConn, taddr *net.TCPAddr) {
	defer serverConn.Close()
	targets := make(map[int32]io.ReadWriteCloser)
	blockReader := bsc.BlockReader{Reader: serverConn}
	for {
		block, err := blockReader.Read()
		if err != nil {
			log.Println("read data channel ", err)
			break
		}
		if block.Type == bsc.TYPE_DATA {
			if target, ok := targets[block.Tag]; ok {
				n, err := target.Write(block.Data)
				if err != nil || n < len(block.Data) {
					if err == nil {
						err = io.ErrShortWrite
					}
					closeTag(serverConn, block.Tag, err)
					delete(targets, block.Tag)
				}
			} else {
				closeTag(serverConn, block.Tag, errors.New("channel target not found"))
			}
			continue
		}
		if block.Type == bsc.TYPE_OPEN {
			log.Println("open channel", block.Tag)
			targetConn, err := dialTarget(taddr)
			if err != nil {
				closeTag(serverConn, block.Tag, err)
				continue
			}
			targets[block.Tag] = targetConn
			go func() {
				io.Copy(bsc.NewBlockWriter(serverConn, block.Tag), targetConn)
				closeTag(serverConn, block.Tag, errors.New("copy done."))
			}()
			continue
		}
		if block.Type == bsc.TYPE_CLOSE {
			log.Println("close channel by server", block.Tag)
			if target, ok := targets[block.Tag]; ok {
				target.Close()
				delete(targets, block.Tag)
			}
			continue
		}
		if block.Type == bsc.TYPE_PING {
			pang(serverConn)
			continue
		}
		log.Println("not support block type", block.Type)
	}
}

//
func main() {
	log.Println("hello bsc-client")
	flag.Parse()
	if *serverAddr == "" {
		flag.PrintDefaults()
		return
	}
	if *domain == "" {
		flag.PrintDefaults()
		return
	}
	if *target == "" {
		flag.PrintDefaults()
		return
	}

	taddr, err := net.ResolveTCPAddr("tcp", *target)
	if err != nil {
		log.Println(err)
		return
	}
	daddr, err := net.ResolveTCPAddr("tcp", *serverAddr)
	if err != nil {
		log.Println(err)
		return
	}
	var retryCount = 0
	for *retry == -1 || retryCount <= *retry {
		conn, err := net.DialTCP("tcp", nil, daddr)
		if err == nil {
			conn.Write([]byte{bsc.TYPE_INIT})
			ben.WriteLDString(conn, *domain)
			ben.WriteLDString(conn, *rhost)
			handConn(conn, taddr)
		}
		retryCount++
		atTime := time.Now().Add(10 * time.Second)
		log.Println("retry connect server @", atTime)
		time.Sleep(10 * time.Second)
	}
}
