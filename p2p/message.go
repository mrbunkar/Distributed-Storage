package p2p

const (
	IncomingMessage = 0x1
	IncomingStream  = 0x2
	FileNotFound    = 0x3
	FileFound       = 0x4
)

type RPC struct {
	From    string
	Payload []byte
	Stream  bool
}
