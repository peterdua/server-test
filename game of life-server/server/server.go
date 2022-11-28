package main

import (
	"flag"
	"net"
	"net/rpc"

	"github.com/apex/log"
	"uk.ac.bris.cs/gameoflife/stubs"
)

type Server struct {
}

func (s *Server) Next(req stubs.NextRequest, res *stubs.NextResponse) (err error) {
	return
}

func main() {
	pAddr := flag.String("port", "8010", "Port to listen on") //8010
	flag.Parse()
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	log.Infof("listening on %s", listener.Addr().String())
	defer listener.Close()
	rpc.Register(new(Server))
	rpc.Accept(listener)
}
