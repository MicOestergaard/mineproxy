package main

import (
	"fmt"
	"log"
	"net"
	"sync"
)

type Proxy struct {
	from string
	to   string
	lock *sync.Mutex
	done chan struct{}
}

func NewProxy(listenPort int) *Proxy {
	return &Proxy{
		from: fmt.Sprintf(":%d", listenPort),
		lock: new(sync.Mutex),
	}
}

func (p *Proxy) Start() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.done != nil {
		return fmt.Errorf("already started")
	}

	p.done = make(chan struct{})
	listener, err := net.Listen("tcp", p.from)
	if err != nil {
		return err
	}
	go p.waitForConnection(listener)

	return nil
}

func (p *Proxy) Stop() error {
	p.lock.Lock()
	defer p.lock.Unlock()

	if p.done == nil {
		return fmt.Errorf("already stopped")
	}

	close(p.done)
	p.done = nil

	return nil
}

func (p *Proxy) SetTarget(target string) {
	p.to = target
}

func (p *Proxy) waitForConnection(listener net.Listener) {
	for {
		select {
		case <-p.done:
			return
		default:
			connection, err := listener.Accept()
			if err != nil {
				log.Println("Error accepting connection:", err)
			} else {
				tcpConnection, ok := connection.(*net.TCPConn)
				if !ok {
					log.Println("Accepted connection is not a tcp connection")
					continue
				}
				go p.handleConnection(tcpConnection)
			}
		}
	}
}

func (p *Proxy) handleConnection(connection net.Conn) {
	remoteAddress := connection.RemoteAddr().String()

	log.Println("Handling connection from", remoteAddress)
	defer log.Println("Finished handling connection from", remoteAddress)
	defer connection.Close()

	if p.to == "" {
		log.Println("Error handling connection: missing target")
		return
	}

	remote, err := net.Dial("tcp", p.to)
	if err != nil {
		log.Println("Error handling connection: error connecting to target: ", err)
		return
	}
	defer remote.Close()
	p.establishPipe(connection, remote)
}

func (p *Proxy) establishPipe(serverConnection, clientConnection net.Conn) {
	serverClosed := make(chan struct{}, 1)
	clientClosed := make(chan struct{}, 1)

	tcpServerConnection, _ := serverConnection.(*net.TCPConn)
	tcpClientConnection, _ := clientConnection.(*net.TCPConn)

	go copyData(tcpServerConnection, tcpClientConnection, clientClosed)
	go copyData(tcpClientConnection, tcpServerConnection, serverClosed)

	var waitChannel chan struct{}
	select {
	case <-clientClosed:
		_ = tcpServerConnection.SetLinger(0)
		_ = tcpServerConnection.CloseRead()
		waitChannel = serverClosed
	case <-serverClosed:
		_ = tcpClientConnection.CloseRead()
		waitChannel = clientClosed
	}

	<-waitChannel
}

func copyData(dst, src *net.TCPConn, srcClosed chan struct{}) {
	_, err := dst.ReadFrom(src)
	if err != nil {
		log.Println("error copying data:", err)
	}

	err = src.Close()
	if err != nil {
		log.Println("error closing source:", err)
	}

	srcClosed <- struct{}{}
}
