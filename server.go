package main

import (
	"bytes"
	"distributed_storage/p2p"
	"encoding/binary"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"log"
	"sync"
	"time"
)

type FileServerOpts struct {
	transport         p2p.Transport
	StorageRoot       string
	bootstrapNode     []string
	PathTransformFunc PathTransformFunc
}

type FileServer struct {
	EncKey []byte
	Opts   FileServerOpts
	store  *Store
	qutich chan struct{}

	LockPeer sync.Mutex
	peers    map[string]p2p.Peer
}

func NewFileServer(opts FileServerOpts) *FileServer {
	StoreOpts := StoreOpts{
		Root:              opts.StorageRoot,
		PathTransformFunc: opts.PathTransformFunc,
	}

	return &FileServer{
		Opts:   opts,
		EncKey: NewEncryptionKey(),
		store:  NewStore(StoreOpts),
		qutich: make(chan struct{}),
		peers:  make(map[string]p2p.Peer),
	}
}

func (fs *FileServer) Stop() {
	log.Println("Stopping the file server ...")
	close(fs.qutich)
}

func (fs *FileServer) loop() {
	defer func() {
		log.Println("File Server stopped due to error or user action ...")
		fs.Opts.transport.Close()
	}()

	for {
		select {
		case rpc := <-fs.Opts.transport.Consume():
			var msg p2p.Message
			fmt.Printf("Got message, %v\n", string(rpc.Payload))

			if err := gob.NewDecoder(bytes.NewReader(rpc.Payload)).Decode(&msg); err != nil {
				fmt.Println(err)
				return
			}

			if err := fs.hanleMessage(rpc.From, &msg); err != nil {
				log.Println(err)
				return
			}
		case <-fs.qutich:
			return
		}
	}
}

func (fs *FileServer) Start() error {
	if err := fs.Opts.transport.ListenAndAccept(); err != nil {
		return err
	}

	fs.BootStrapingNetwork()
	fs.loop()
	return nil
}

func (fs *FileServer) Get(key string) (int, io.Reader, error) {
	if fs.store.HasKey(key) {
		log.Println("Got file locally")
		return fs.store.Read(key)
	}

	log.Println("Dont have file locally, fetching from the peer")
	msg := p2p.Message{
		Payload: p2p.MessageGetFile{
			Key: key,
		},
	}

	if err := fs.Broadcast(msg); err != nil {
		return 0, nil, err
	}
	time.Sleep(time.Millisecond * 100)

	fileFound := false

	for _, peer := range fs.peers {
		log.Printf("[%s] reading from peer \n", peer.RemoteAddr().String())
		peekBuf := make([]byte, 1)

		if _, err := peer.Read(peekBuf); err != nil {
			log.Printf("Error reading Peek Buffer from peer [%s] \n", peer.RemoteAddr().String())
			continue
		}

		if peekBuf[0] != p2p.FileFound {
			log.Printf("File not found on the peer [%s] \n", peer.RemoteAddr().String())
			continue
		}
	}

	if fileFound == false {
		log.Printf("File with key [%s] not found on any peer \n", key)
		return 0, nil, errors.New("File not found")
	}

	time.Sleep(time.Millisecond * 10)
	return fs.store.Read(key)
}

func (fs *FileServer) OnPeer(peer p2p.Peer) error {
	fs.LockPeer.Lock()
	defer fs.LockPeer.Unlock()

	fs.peers[peer.RemoteAddr().String()] = peer
	log.Printf("Connected to peer %s \n", peer.RemoteAddr().String())
	return nil
}

func (fs *FileServer) hanleMessage(from string, msg *p2p.Message) error {
	switch v := msg.Payload.(type) {
	case p2p.MessageGetFile:
		return fs.hanleMessageGetFile(from, &v)
	case p2p.MessageStoreFile:
		return fs.hanleMessageStoreFile(from, &v)
	}
	return nil
}

func (fs *FileServer) hanleMessageGetFile(from string, msg *p2p.MessageGetFile) error {
	peer, ok := fs.peers[from]
	if !ok {
		return fmt.Errorf("Peer could not be found n the peer list")
	}

	if !fs.store.HasKey(msg.Key) {
		peer.Write([]byte{p2p.IncomingStream})
		peer.Write([]byte{p2p.FileNotFound})
		return nil
	}

	fmt.Println("Got file, serving over the nwtwork...")
	size, reader, err := fs.store.Read(msg.Key)
	rc, ok := reader.(io.ReadCloser)
	if ok {
		fmt.Println("Closing the read closer")
		defer rc.Close()
	}

	if err != nil {
		return err
	}

	peer.Write([]byte{p2p.IncomingStream})
	peer.Write([]byte{p2p.FileFound})

	//  Sending the file size over the network
	if err := binary.Write(peer, binary.LittleEndian, size); err != nil {
		return fmt.Errorf("Failed to write file size to peer: %w", err)
	}

	time.Sleep(time.Millisecond * 10)

	n, err := io.Copy(peer, rc)
	if err != nil {
		return err
	}

	log.Printf("[%d] Byte send over to peer: [%s] \n", n, peer.RemoteAddr().String())
	return nil
}

func (fs *FileServer) hanleMessageStoreFile(from string, msg *p2p.MessageStoreFile) error {

	peer, ok := fs.peers[from]
	defer peer.CloseStream()
	if !ok {
		return fmt.Errorf("Peer could not be found n the peer list")
	}

	n, err := fs.store.Write(msg.Key, io.LimitReader(peer, msg.Size))

	if err != nil {
		return err
	}

	log.Printf("[%d] Bytes written to disk \n", n)
	return nil
}

func (fs *FileServer) Store(key string, r io.Reader) error {
	//  Store the file in DB and broadcast the msg
	var (
		fileBuf = new(bytes.Buffer)
		tee     = io.TeeReader(r, fileBuf)
	)

	size, err := fs.store.Write(key, tee)
	if err != nil {
		return err
	}
	fmt.Printf("[%d] bytes Written to disk, now sending over the network \n", size)

	msg := p2p.Message{
		Payload: p2p.MessageStoreFile{
			Key:  key,
			Size: size,
		},
	}

	if err := fs.Broadcast(msg); err != nil {
		return err
	}
	for _, peer := range fs.peers {
		peer.Write([]byte{p2p.IncomingStream})
		n, err := io.Copy(peer, fileBuf)
		if err != nil {
			return err
		}
		fmt.Printf("[%d] bytes send over the network", n)
	}

	return nil
}

func (fs *FileServer) Broadcast(msg p2p.Message) error {
	msgBuf := new(bytes.Buffer)

	if err := gob.NewEncoder(msgBuf).Encode(msg); err != nil {
		return err
	}

	for _, peer := range fs.peers {
		peer.Write([]byte{p2p.IncomingMessage})
		peer.Write(msgBuf.Bytes())
	}

	return nil
}

func init() {
	gob.Register(p2p.MessageGetFile{})
	gob.Register(p2p.MessageStoreFile{})
}

func (fs *FileServer) BootStrapingNetwork() {
	for _, addr := range fs.Opts.bootstrapNode {
		if len(addr) == 0 {
			continue
		}

		log.Println("Dialing to node", addr)
		go func(addr string) {
			if err := fs.Opts.transport.Dial(addr); err != nil {
				log.Println("Error dialing to addr:", addr)
			}
		}(addr)

	}
}

func (fs *FileServer) Stream(msg *p2p.Message) error {
	peers := []io.Writer{}

	for _, peer := range fs.peers {
		peers = append(peers, peer)
	}

	mw := io.MultiWriter(peers...)

	return gob.NewEncoder(mw).Encode(msg)
}
