package main

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
	"sync"

	bsc "github.com/muyuballs/bsc/v2"
)

type Client struct {
	Domain  string
	Rewrite string
	Service *net.TCPConn
	pipMap  map[int]*io.PipeWriter
	locker  sync.Mutex
	ref     int
}

func NewClient(domain, rewrite string, svr *net.TCPConn) *Client {
	return &Client{
		Domain:  domain,
		Rewrite: rewrite,
		Service: svr,
		locker:  sync.Mutex{},
	}
}

func (c *Client) CloseDataChannel(channel *DataChannel) {
	defer c.locker.Unlock()
	c.locker.Lock()
	c.ref--
	if w, ok := c.pipMap[channel.SID]; ok {
		delete(c.pipMap, channel.SID)
		w.Close()
	}
}

func (c *Client) Close() {
	if c.Service != nil {
		c.Service.Close()
		c.Service = nil
	}
}

func (c *Client) SendPingMessage() (err error) {
	_, err = c.Service.Write([]byte{bsc.C_TYPE_P})
	return
}

func (c Client) StartSerivce() {
	go func() {
		defer c.Close()
		reader := bsc.BlockReader{Reader: bufio.NewReader(c.Service)}
		for {
			block, err := reader.Read()
			if err != nil {
				log.Println("read domain channel ", err)
				break
			}
			if writer, ok := c.pipMap[block.Tag]; ok {
				n, err := writer.Write(block.Data)
				if err != nil || n < len(block.Data) {
					if err != nil {
						log.Println(err)
					} else {
						log.Println(io.ErrShortWrite)
					}
					delete(c.pipMap, block.Tag)
				}
			}
		}
	}()
}

func (c *Client) CreateDataChannel() *DataChannel {
	defer c.locker.Unlock()
	c.locker.Lock()
	c.ref++
	r, w := io.Pipe()
	c.pipMap[c.ref] = w
	return &DataChannel{
		SID:    c.ref,
		Rhost:  c.Rewrite,
		Writer: bsc.NewBlockWriter(c.Service, c.ref),
		Reader: r,
		Mgr:    c,
	}
}

func (c *Client) Transfer(w http.ResponseWriter, r *http.Request) {
	channel := c.CreateDataChannel()
	if channel == nil {
		bsc.ServiceUnavailable(w, r)
	}
	channel.Transfer(w, r)
}
