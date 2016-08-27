package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"time"

	bsc "github.com/muyuballs/bsc/v2"
)

const (
	DC_MAX_REF              = 100
	DOMAIN_CHANNEL_TIME_OUT = time.Second * 15
)

type DomainChannel struct {
	ref     int
	Ctag    int
	Rhost   string
	Writer  *net.TCPConn
	CWriter map[byte]io.WriteCloser
}

func (dc *DomainChannel) Close() {
	dc.Writer.Close()
	for _, w := range dc.CWriter {
		w.Close()
	}
}

func (dc *DomainChannel) CloseDataChannel(datac *DataChannel) {
	dc.CloseTag(datac.Tag)
	delete(dc.CWriter, datac.Tag)
	dc.ref--
}

func (dc *DomainChannel) Start(reader bsc.BlockReader) {
	rc := make(chan int)
	go func() {
		defer dc.Close()
		for {
			block, err := reader.Read()
			if err != nil {
				log.Println("read domain channel ", err)
				break
			}
			rc <- 1
			if writer, ok := dc.CWriter[block.Tag]; ok {
				n, err := writer.Write(block.Data)
				if err != nil || n < len(block.Data) {
					if err != nil {
						log.Println(err)
					} else {
						log.Println(io.ErrShortWrite)
					}
					delete(dc.CWriter, block.Tag)
				}
			}
		}
		rc <- 0
	}()
	go func() {
		defer dc.Close()
		for {
			select {
			case p := <-rc:
				if p == 0 {
					break
				}
			case <-time.Tick(DOMAIN_CHANNEL_TIME_OUT):
				break
			}
		}
		log.Println("close domain channel")
	}()
}

func (dc *DomainChannel) CloseTag(tag byte) {
	block := bsc.Block{Tag: tag, Type: bsc.BL_TYPE_CLOSE}
	block.WriteTo(dc.Writer)
}

func (dc *DomainChannel) OpenTag(tag byte) {
	block := bsc.Block{Tag: tag, Type: bsc.BL_TYPE_OPEN}
	block.WriteTo(dc.Writer)
}

func (dc *DomainChannel) NewDataChannel() *DataChannel {
	if dc.ref >= DC_MAX_REF {
		log.Println("domain reatch max datachannel count")
		return nil
	}
	log.Println("create data channel")
	dc.ref++
	dc.Ctag++
	if dc.Ctag > 125 {
		dc.Ctag = 0
	}
	r, w := io.Pipe()
	channel := &DataChannel{
		Tag:           byte(dc.Ctag),
		Rhost:         dc.Rhost,
		Writer:        bsc.NewBlockWriter(dc.Writer, byte(dc.Ctag)),
		Reader:        r,
		domainChannel: dc,
	}
	dc.CWriter[channel.Tag] = w
	dc.OpenTag(channel.Tag)
	return channel
}

func CreateDomainChannel(conn *net.TCPConn) (dc *DomainChannel, err error) {
	log.Println("create domain channel")
	dc = &DomainChannel{
		Writer:  conn,
		ref:     0,
		Ctag:    0,
		CWriter: make(map[byte]io.WriteCloser),
	}
	dc.Start(bsc.BlockReader{Reader: bufio.NewReader(conn)})
	return
}
