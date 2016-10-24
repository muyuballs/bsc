package main

import (
	"bufio"
	"errors"
	"io"
	"log"
	"net"

	bsc "github.com/muyuballs/bsc/v2"
)

const (
	HTTP = 0
	TCP  = 1
)

func closeTag(bscConn *net.TCPConn, tag int32, err error) {
	log.Println("close channel", tag, err)
	b := bsc.Block{Tag: tag, Type: bsc.TYPE_CLOSE}
	b.WriteTo(bscConn)
}

func pang(bscConn *net.TCPConn) {
	b := bsc.Block{Type: bsc.TYPE_PANG}
	b.WriteTo(bscConn)
}

func handConn(tType int, bscConn *net.TCPConn, targetAddr *net.TCPAddr) {
	defer bscConn.Close()
	blockReader := bsc.BlockReader{Reader: bufio.NewReader(bscConn)}
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
					delete(targets, block.Tag)
					target.Close()
					if err == nil {
						err = io.ErrShortWrite
					}
					closeTag(bscConn, block.Tag, err)
				}
			} else {
				closeTag(bscConn, block.Tag, errors.New("channel target not found"))
			}
			continue
		}
		if block.Type == bsc.TYPE_OPEN {
			log.Println("open channel", block.Tag)
			if target, ok := targets[block.Tag]; ok {
				delete(targets, block.Tag)
				target.Close()
				closeTag(bscConn, block.Tag, errors.New("close finished channel"))
			}
			if tType == HTTP {
				openHTTPChannel(bscConn, block.Tag, targetAddr)
			} else if tType == TCP {
				openTCPChannel(bscConn, block.Tag, targetAddr)
			}
			continue
		}
		if block.Type == bsc.TYPE_CLOSE {
			log.Println("close channel by server", block.Tag)
			if target, ok := targets[block.Tag]; ok {
				delete(targets, block.Tag)
				target.Close()
			}
			continue
		}
		if block.Type == bsc.TYPE_PING {
			log.Println("Ping from ", bscConn.LocalAddr().String())
			pang(bscConn)
			continue
		}
		if block.Type == bsc.TYPE_PANG {
			log.Println("Pang from ", bscConn.LocalAddr().String())
			continue
		}
		log.Println("not support block type", block.Type)
	}
}
