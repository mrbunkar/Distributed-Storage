package main

import (
	"distributed_storage/p2p"
	"fmt"
	"strings"
)

func MakeServer(listenAddr string, nodes []string) *FileServer {
	tcp_opts := p2p.TCPTransportOpts{
		ListenAddress: listenAddr,
		HandshakeFunc: p2p.NOPHandShake,
		Decoder:       p2p.DefaultDecoder{},
	}
	transport := p2p.TCPTransport{
		Opts: tcp_opts,
	}

	server_opts := FileServerOpts{
		StorageRoot: fmt.Sprintf("%s_files", strings.Split(listenAddr, ":")[1]),
		transport:   transport,
	}
}

func main() {
	s1 := MakeServer(":3000", []string{})
	s2 := MakeServer(":4000", []string{":3000"})
}
