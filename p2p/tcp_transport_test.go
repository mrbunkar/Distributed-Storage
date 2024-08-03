package p2p

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTcpTransportTest(t *testing.T) {
	opts := TCPTransportOpts{
		ListenAddress: ":3000",
		HandshakeFunc: NOPHandShake,
		Decoder:       DefaultDecoder{},
	}
	tcp := NewTCPTransport(opts)
	assert.Equal(t, opts.ListenAddress, ":3000")
	assert.Nil(t, tcp.ListenAndAccept())
}
