package main

import (
	"flag"
	"fmt"
	"io"
	"log"
	"strconv"
)

type flagInt32 int32

var (
	serverAddr string
	target     string
	retry      int
	tunType    string
	domain     string
	rhost      string
	isTLS      bool
	tcpPort    flagInt32

	targets = make(map[int32]io.ReadWriteCloser)
)

func (c *flagInt32) Set(val string) (err error) {
	v, err := strconv.ParseInt(val, 10, 32)
	if err == nil {
		*c = flagInt32(v)
	}
	return
}

func (c *flagInt32) String() string {
	return fmt.Sprintf("%d", *c)
}

//
func main() {
	log.Println("hello bsc-client")
	flag.StringVar(&serverAddr, "server", "", "bsc server address")
	flag.StringVar(&tunType, "type", "http", "tun type (http/tcp), default http")
	flag.StringVar(&target, "target", "", "target service address")
	flag.IntVar(&retry, "retry count", -1, "retry count default -1 for ever")
	flag.StringVar(&domain, "domain", "", "handle domain")
	flag.StringVar(&rhost, "rhost", "", "rewrite domain to host")
	flag.BoolVar(&isTLS, "tls", false, "is https")
	flag.Var(&tcpPort, "pport", "bsc server shulde exported tcp port")
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
		handHTTPProxy()
	} else if tunType == "tcp" {
		handTCPTun()
	} else {
		log.Println("not supported tun type")
		flag.PrintDefaults()
	}

}
