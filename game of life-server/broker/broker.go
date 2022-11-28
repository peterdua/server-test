package main

import (
	"flag"
	"net"
	"net/rpc"

	"github.com/apex/log"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type Broker struct {
}

func (b *Broker) GameOfLife(req stubs.GolRequest, res *stubs.GolResponse) (err error) {
	return
}

func (b *Broker) KeyPress(req stubs.KeyPressRequest, res *stubs.KeyPressResponse) (err error) {
	return
}

func main() {
	pAddr := flag.String("port", "8012", "Port to listen on") //8012
	flag.Parse()
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Infof("listening on %s", listener.Addr().String())
	defer listener.Close()

	broker := &Broker{}
	rpc.Register(broker)
	rpc.Accept(listener)
}
