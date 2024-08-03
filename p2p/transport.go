package p2p

import "net"

type Peer interface {
	net.Conn
	Close() error
	CloseStream()
}

type Transport interface {
	ListenAndAccept() error
	Consume() <-chan RPC
	Close() error
	Dial(string) error
}
