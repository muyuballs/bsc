package main

import (
	"flag"
	"io"
	"log"
	"net"

	bsc "github.com/muyuballs/bsc/v2"
	"github.com/muyuballs/bsc/v2/ben"
)

var serverAddr = flag.String("server", "", "bsc server addr")
var domain = flag.String("domain", "", "service public domain")
var rhost = flag.String("rhost", "", "host rewrite to")
var target = flag.String("target", "", "target service addr")

var (
	CWriter = make(map[int]io.Writer)
)

func openConn(taddr *net.TCPAddr) (conn *net.TCPConn, err error) {
	log.Println("dial taddr", taddr)
	dConn, err := net.DialTCP("tcp", nil, taddr)
	if err != nil {
		log.Println("dial taddr:", err)
		return
	}
	log.Println("dial taddr done.")
	return dConn, nil
}

func handConn(dConn *net.TCPConn, taddr *net.TCPAddr) {
	defer dConn.Close()
	CWriter = make(map[int]io.Writer)
	blockReader := bsc.BlockReader{Reader: dConn}
	for {
		block, err := blockReader.Read()
		if err != nil {
			log.Println("read data channel ", err)
			break
		}
		if block.Type == bsc.BL_TYPE_CLOSE {
			log.Println("close channel", block.Tag)
			delete(CWriter, block.Tag)
			continue
		}
		if block.Type == bsc.BL_TYPE_OPEN {
			log.Println("open channel", block.Tag)
			tConn, err := openConn(taddr)
			if err != nil {
				continue
			}
			go io.Copy(bsc.NewBlockWriter(dConn, block.Tag), tConn)
			CWriter[block.Tag] = tConn
		}
		if writer, ok := CWriter[block.Tag]; ok {
			n, err := writer.Write(block.Data)
			if err != nil || n < len(block.Data) {
				if err != nil {
					log.Println(err)
				} else {
					log.Println(io.ErrShortWrite)
				}
				delete(CWriter, block.Tag)
			}
		}
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
	conn, err := net.DialTCP("tcp", nil, daddr)
	if err != nil {
		log.Println(err)
		return
	}
	conn.Write([]byte{bsc.C_TYPE_C})
	ben.WriteLDString(conn, *domain)
	ben.WriteLDString(conn, *rhost)
	handConn(conn, taddr)
}
