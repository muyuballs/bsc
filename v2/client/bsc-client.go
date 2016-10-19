package main

import (
	"flag"
	"io"
	"log"
)

var (
	serverAddr string
	target     string
	retry      int
	tunType    string
	domain     string
	rhost      string
	isTls      bool
	tcpPort    int

	targets = make(map[int32]io.ReadWriteCloser)
)

//
func main() {
	log.Println("hello bsc-client")
	flag.StringVar(&serverAddr, "server", "", "bsc server address")
	flag.StringVar(&tunType, "type", "http", "tun type (http/tcp), default http")
	flag.StringVar(&target, "target", "", "target service address")
	flag.IntVar(&retry, "retry count", -1, "retry count default -1 for ever")
	flag.StringVar(&domain, "domain", "", "handle domain")
	flag.StringVar(&rhost, "rhost", "", "rewrite domain to host")
	flag.BoolVar(&isTls, "tls", false, "is https")
	flag.IntVar(&tcpPort, "pport", 0, "bsc server shulde exported tcp port")
	flag.Parse()
	if serverAddr == "" {
		log.Println("server must not null")
		flag.PrintDefaults()
		return
	}
	if target == "" {
		log.Println("target must not null")
		flag.PrintDefaults()
		return
	}

	if tunType == "http" {
		handHttpTun()
	} else if tunType == "tcp" {
		handTcpTun()
	} else {
		log.Println("not supported tun type")
		flag.PrintDefaults()
	}

}
