package p2p

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

type TCPPeer struct {
	net.Conn
	outbound bool
	wg       sync.WaitGroup
}

type TCPTransportOpts struct {
	ListenAddress string
	Decoder       Decoder
	HandshakeFunc HandshakeFunc
	OnPeer        func(Peer) error
}

type TCPTransport struct {
	Opts     TCPTransportOpts
	listener net.Listener
	rpcch    chan RPC
}

func (p *TCPPeer) CloseStream() {
	p.wg.Done()
}

func NewTCPPeer(conn net.Conn, outbound bool) *TCPPeer {
	return &TCPPeer{
		Conn:     conn,
		outbound: outbound,
	}
}

func NewTCPTransport(opts TCPTransportOpts) *TCPTransport {
	return &TCPTransport{
		Opts:  opts,
		rpcch: make(chan RPC),
	}
}

func (p *TCPPeer) Close() error {
	return p.Conn.Close()
}

// Consume implements Transport interface, which will retunn a channel for
// reading the incoming msg from another peer
func (t *TCPTransport) Consume() <-chan RPC {
	return t.rpcch
}

func (t *TCPTransport) ListenAndAccept() error {
	var err error
	t.listener, err = net.Listen("tcp", t.Opts.ListenAddress)

	if err != nil {
		return err
	}

	go t.startAcceptListen()
	log.Printf("TCP Transport is listening on %s\n", t.Opts.ListenAddress)
	return nil
}

func (t *TCPTransport) startAcceptListen() {
	for {
		conn, err := t.listener.Accept()

		if errors.Is(err, net.ErrClosed) {
			return
		}

		if err != nil {
			fmt.Printf("TCP: accept error %v\n", err)
		}

		log.Println("Incoming connection from server", conn.RemoteAddr())
		go t.handleConn(conn, false)
	}
}

func (t *TCPTransport) handleConn(conn net.Conn, outbound bool) {
	var err error

	defer func() {
		fmt.Println("Dropping peer connection")
		conn.Close()
	}()

	peer := NewTCPPeer(conn, outbound)

	if err = t.Opts.HandshakeFunc(peer); err != nil {
		fmt.Printf("TCP handshake error: %v\n", err)
		return
	}

	if t.Opts.OnPeer != nil {
		if err = t.Opts.OnPeer(peer); err != nil {
			fmt.Printf("TCP on peer connnection error: %v\n", err)
			return
		}
	}

	for {
		rpc := RPC{}

		if err := t.Opts.Decoder.Decode(conn, &rpc); err != nil {
			fmt.Printf("TCP handle connection error %v\n", err)

			if err == io.EOF {
				fmt.Printf("Connection closed by peer: %v\n", conn.RemoteAddr())
				break
			}
			continue
		}
		rpc.From = conn.RemoteAddr().String()

		if rpc.Stream {
			peer.wg.Add(1)
			fmt.Printf("[%s] Incoming stream. waiting till stream is done... \n", rpc.From)
			peer.wg.Wait()
			fmt.Printf("[%s] stream closed,resuming read loop \n", rpc.From)
		}
		fmt.Printf("Incoming message from [%s] \n", rpc.From)
		t.rpcch <- rpc
	}
}

func (t *TCPTransport) Close() error {
	return t.listener.Close()
}

func (t *TCPTransport) Dial(addr string) error {
	conn, err := net.Dial("tcp", addr)

	if err != nil {
		return err
	}

	go t.handleConn(conn, true)
	return nil
}
