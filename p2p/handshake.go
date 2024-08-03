package p2p

type HandshakeFunc func(Peer) error

func NOPHandShake(Peer) error { return nil }
