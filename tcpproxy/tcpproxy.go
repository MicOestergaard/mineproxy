package tcpproxy

import (
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

type Proxy struct {
	from string
	to   string
	lock *sync.RWMutex
	done chan struct{}
}

func NewProxy(listenPort int) *Proxy {
	return &Proxy{
		from: fmt.Sprintf(":%d", listenPort),
		lock: new(sync.RWMutex),
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
	p.lock.Lock()
	defer p.lock.Unlock()
	p.to = target
}

func (p *Proxy) getTarget() string {
	p.lock.RLock()
	defer p.lock.RUnlock()
	return p.to
}

func (p *Proxy) waitForConnection(listener net.Listener) {
	for {
		select {
		case <-p.done:
			listener.Close()
			return
		default:
			connection, err := listener.Accept()
			if err != nil {
				if !isClosedError(err) {
					log.Println("Error accepting connection:", err)
				}
			} else {
				tcpConnection, ok := connection.(*net.TCPConn)
				if !ok {
					log.Println("Accepted connection is not a tcp connection")
					connection.Close()
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

	target := p.getTarget()
	if target == "" {
		log.Println("Error handling connection: missing target")
		return
	}

	remote, err := net.DialTimeout("tcp", target, 10*time.Second)
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
		if err := tcpServerConnection.SetLinger(0); err != nil {
			log.Printf("Error setting linger: %v", err)
		}
		if err := tcpServerConnection.CloseRead(); err != nil {
			log.Printf("Error closing read: %v", err)
		}
		waitChannel = serverClosed
	case <-serverClosed:
		if err := tcpClientConnection.CloseRead(); err != nil {
			log.Printf("Error closing read: %v", err)
		}
		waitChannel = clientClosed
	}

	<-waitChannel
}

func copyData(dst, src *net.TCPConn, srcClosed chan struct{}) {
	buf := make([]byte, 32*1024)
	_, err := io.CopyBuffer(dst, src, buf)
	if err != nil && !isClosedError(err) {
		log.Printf("Error copying data: %v", err)
	}

	if err := src.Close(); err != nil {
		log.Printf("Error closing source: %v", err)
	}

	srcClosed <- struct{}{}
}

func isClosedError(err error) bool {
	if err == nil {
		return false
	}
	return err.Error() == "use of closed network connection"
}
