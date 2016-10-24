package main

import (
	"bufio"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"sync"
	"time"

	bsc "github.com/muyuballs/bsc/v2"
)

type Client struct {
	Type    int
	Domain  string
	Rewrite string
	Service *net.TCPConn
	pipMap  map[int32]*io.PipeWriter
	lk      sync.Mutex
	r       *rand.Rand
}

func NewClient(domain, rewrite string, svr *net.TCPConn) *Client {
	return &Client{
		Type:    bsc.TYPE_HTTP,
		Domain:  domain,
		Rewrite: rewrite,
		Service: svr,
		lk:      sync.Mutex{},
		pipMap:  make(map[int32]*io.PipeWriter),
		r:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

func NewTcpClient(port int, svr *net.TCPConn) *Client {
	return &Client{
		Type:    bsc.TYPE_TCP,
		Service: svr,
		lk:      sync.Mutex{},
		pipMap:  make(map[int32]*io.PipeWriter),
	}
}

func (c *Client) CloseDataChannel(channel *DataChannel) {
	defer c.lk.Unlock()
	c.lk.Lock()
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

func (c *Client) StartSerivce() {
	go func() {
		defer c.Remove()
		reader := bsc.BlockReader{Reader: bufio.NewReader(c.Service)}
		for {
			block, err := reader.Read()
			if err != nil {
				log.Println("read domain channel ", err)
				break
			}
			switch block.Type {
			case bsc.TYPE_DATA:
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
			case bsc.TYPE_CLOSE:
				if writer, ok := c.pipMap[block.Tag]; ok {
					writer.Close()
					delete(c.pipMap, block.Tag)
				}
			case bsc.TYPE_PANG:
			case bsc.TYPE_PING:
				pingBlock := bsc.Block{Type: bsc.TYPE_PANG}
				pingBlock.WriteTo(c.Service)
			default:
				log.Println("not support block type", block.Type)
			}
		}
	}()
}

func (c *Client) CreateDataChannel() *DataChannel {
	defer c.lk.Unlock()
	c.lk.Lock()
	sid := c.r.Int31()
	r, w := io.Pipe()
	c.pipMap[sid] = w
	return &DataChannel{
		SID:   sid,
		Rhost: c.Rewrite,
		//Writer: bsc.NewBlockWriter(bsc.NewTrackWriter(CH_C_OUT, c.Service), sid),
		Writer: bsc.NewBlockWriter(c.Service, sid),
		//Reader: bsc.NewTrackReader(CH_C_IN, r),
		Reader: r,
		Mgr:    c,
	}
}

func (c *Client) Transfer(w http.ResponseWriter, r *http.Request, st time.Time) {
	channel := c.CreateDataChannel()
	if channel == nil {
		bsc.ServiceUnavailable(w, r)
		return
	}
	channel.Transfer(w, r, st)
}

func (c *Client) TransferTcp(conn *net.TCPConn) {
	channel := c.CreateDataChannel()
	if channel == nil {
		conn.Close()
		c.Close()
		return
	}
	channel.TransferTcp(conn)
}
