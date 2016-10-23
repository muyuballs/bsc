package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"time"

	bsc "github.com/muyuballs/bsc/v2"
)

type DataChannelManager interface {
	CloseDataChannel(*DataChannel)
}

type DataChannel struct {
	SID    int32
	Rhost  string
	Writer bsc.BlockWriter
	Reader io.ReadCloser
	Mgr    DataChannelManager
}

func (dc *DataChannel) Close() {
	dc.Mgr.CloseDataChannel(dc)
	dc.Writer.WriteBlock(bsc.TYPE_CLOSE, nil)
	dc.Writer.Close()
	dc.Reader.Close()
}

func (dc *DataChannel) Write(dat []byte) {
	dc.Writer.WriteBlock(bsc.TYPE_DATA, dat)
}

func (dc *DataChannel) Transfer(w http.ResponseWriter, r *http.Request, handReqTime time.Time) {
	log.Println("start transfer")
	defer log.Println("transfer done")
	defer dc.Close()
	inServerCost := time.Since(handReqTime)
	startTime := time.Now()
	dc.Writer.WriteBlock(bsc.TYPE_OPEN, nil)
	fmt.Fprintf(dc.Writer, "%s %s %s\r\n", r.Method, r.RequestURI, r.Proto)
	if dc.Rhost != "" {
		log.Println("rewrite host", r.Host, "-->", dc.Rhost)
		fmt.Fprintf(dc.Writer, "Host: %s\r\n", dc.Rhost)
	} else {
		fmt.Fprintf(dc.Writer, "Host: %s\r\n", r.Host)
	}
	r.Header.WriteSubset(dc.Writer, map[string]bool{"Host": true})
	dc.Write([]byte("\r\n"))
	if r.Body != nil {
		_, err := io.Copy(dc.Writer, r.Body)
		if err != nil {
			log.Println("copy request body", err)
			return
		}
		dc.Write([]byte("\r\n"))
	}
	copyReqDone := time.Since(startTime)
	log.Println("request body copy done")
	log.Println("start copy response")
	startReadTime := time.Now()
	reader := bufio.NewReader(dc.Reader)
	resp, err := http.ReadResponse(reader, r)
	getRespHeaderDone := time.Since(startReadTime)
	if err != nil {
		log.Println("read response", err)
		return
	}
	log.Println("read response done")
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.Header().Add("X-Bsc-InServer", inServerCost.String())
	w.Header().Add("X-Bsc-WriteReq", copyReqDone.String())
	w.Header().Add("X-Bsc-ReadResp", getRespHeaderDone.String())
	w.WriteHeader(resp.StatusCode)
	if resp.Body != nil {
		io.CopyBuffer(w, resp.Body, make([]byte, 8*1024))
	}
}

func (dc *DataChannel) TransferTcp(conn *net.TCPConn) {
	log.Println("start transfer")
	defer log.Println("transfer done")
	defer dc.Close()
	dc.Writer.WriteBlock(bsc.TYPE_OPEN, nil)
	go io.CopyBuffer(dc.Writer, conn, make([]byte, 8*1024))
	io.CopyBuffer(conn, dc.Reader, make([]byte, 8*1024))
}
