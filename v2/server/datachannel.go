package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
)

type DataChannel struct {
	Tag           byte
	Rhost         string
	Writer        io.WriteCloser
	Reader        io.ReadCloser
	domainChannel *DomainChannel
}

func (dc *DataChannel) Close() {
	dc.domainChannel.CloseDataChannel(dc)
	dc.Writer.Close()
	dc.Reader.Close()
}

func (dc *DataChannel) Transfer(w http.ResponseWriter, r *http.Request) {
	log.Println("start transfer")
	defer log.Println("transfer done")
	defer dc.Close()
	go func() {
		dc.Writer.Write([]byte(fmt.Sprintf("%s %s %s\r\n", r.Method, r.RequestURI, r.Proto)))
		if dc.Rhost != "" {
			log.Println("rewrite host", r.Host, "-->", dc.Rhost)
			dc.Writer.Write([]byte(fmt.Sprintf("Host: %s\r\n", dc.Rhost)))
		} else {
			dc.Writer.Write([]byte(fmt.Sprintf("Host: %s\r\n", r.Host)))
		}
		r.Header.WriteSubset(dc.Writer, map[string]bool{"Host": true})
		dc.Writer.Write([]byte("\r\n"))
		_, err := io.Copy(dc.Writer, r.Body)
		if err != nil {
			log.Println("copy request body", err)
			return
		}
		dc.Writer.Write([]byte("\r\n"))
		log.Println("request body copy done")
	}()
	log.Println("start copy response")
	reader := bufio.NewReader(dc.Reader)
	resp, err := http.ReadResponse(reader, r)
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
	w.WriteHeader(resp.StatusCode)
	if resp.Body != nil {
		io.CopyBuffer(w, resp.Body, make([]byte, 8*1024))
	}
}
