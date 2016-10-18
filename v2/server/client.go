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
	pipMap  map[int32]*io.PipeWriter
	locker  sync.Mutex
	ref     int32
}

func NewClient(domain, rewrite string, svr *net.TCPConn) *Client {
	return &Client{
		Domain:  domain,
		Rewrite: rewrite,
		Service: svr,
		locker:  sync.Mutex{},
		pipMap:  make(map[int32]*io.PipeWriter),
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

func (c *Client) Remove() {
	c.Close()
	clientMap.Remove(c.Domain)
}

func (c *Client) SendPingMessage() (err error) {
	block := &bsc.Block{
		Tag:  0,
		Type: bsc.TYPE_PING,
	}
	_, err = block.WriteTo(c.Service)
	return
}

func (c Client) StartSerivce() {
	go func() {
		defer c.Remove()
		reader := bsc.BlockReader{Reader: bufio.NewReader(c.Service)}
		for {
			block, err := reader.Read()
			if err != nil {
				log.Println("read domain channel ", err)
				break
			}
			if block.Type == bsc.TYPE_DATA {
				if writer, ok := c.pipMap[block.Tag]; ok {
					n, err := writer.Write(block.Data)
					if err != nil || n < len(block.Data) {
						if err != nil {
							log.Println(err)
						} else {
							log.Println(io.ErrShortWrite)
						}
						writer.Close()
						delete(c.pipMap, block.Tag)
					}
				}
			}
			if block.Type == bsc.TYPE_CLOSE {
				if writer, ok := c.pipMap[block.Tag]; ok {
					writer.Close()
					delete(c.pipMap, block.Tag)
				}
				continue
			}
			if block.Type == bsc.TYPE_PANG {
				log.Println("Pong from", c.Service.RemoteAddr().String())
				continue
			}
			log.Println("not support block type", block.Type)
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
		return
	}
	channel.Transfer(w, r)
}
