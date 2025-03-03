package multicastlistener

import (
	"context"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/ipv4"
)

const (
	maxDatagramSize = 1024
)

var (
	regex    *regexp.Regexp
	regexErr error
)

func init() {
	regex, regexErr = regexp.Compile(`\[MOTD]([^\[\]]*)\[/MOTD].*\[AD]([^\[\]]*)\[/AD]`)
	if regexErr != nil {
		log.Fatalf("Failed to compile regex: %v", regexErr)
	}
}

type Config struct {
	ListenAddress string
	BufferSize    int
	RetryDelay    time.Duration
	Interface     *net.Interface
}

type MinecraftMulticastListener struct {
	config    Config
	conn      *net.UDPConn
	lock      *sync.Mutex
	stop      bool
	stopped   chan struct{}
	gameFound func(motd, serverIP string, serverPort int)
	ctx       context.Context
	cancel    context.CancelFunc
}

func NewMinecraftMulticastListener(config Config, gameFound func(serverMOTD, serverIP string, serverPort int)) *MinecraftMulticastListener {
	if config.BufferSize == 0 {
		config.BufferSize = maxDatagramSize
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = time.Second
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &MinecraftMulticastListener{
		config:    config,
		lock:      new(sync.Mutex),
		stop:      true,
		stopped:   make(chan struct{}),
		gameFound: gameFound,
		ctx:       ctx,
		cancel:    cancel,
	}
}

func parseMulticastMessage(b []byte) (string, int, error) {
	m := string(b)

	if !regex.MatchString(m) {
		return "", 0, fmt.Errorf("error parsing multicast message")
	}

	matches := regex.FindStringSubmatch(m)
	motd := matches[1]
	port, err := strconv.Atoi(matches[2])
	if err != nil {
		return "", 0, fmt.Errorf("error parsing multicast message: port number is not an integer")
	}

	return motd, port, nil
}

func parseAddress(addr *net.UDPAddr) string {
	remoteStr := addr.String()
	portSeparator := strings.Index(remoteStr, ":")
	if portSeparator < 0 {
		return remoteStr
	} else {
		return remoteStr[:portSeparator]
	}
}

func listenMulticastUDP(network string, iface *net.Interface, groupAddr *net.UDPAddr) (*net.UDPConn, error) {
	conn, err := net.ListenUDP(network, groupAddr)
	if err != nil {
		return nil, err
	}

	pc := ipv4.NewPacketConn(conn)
	err = pc.JoinGroup(iface, groupAddr)
	if err != nil {
		return nil, err
	}

	err = pc.SetMulticastLoopback(true)
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func (m *MinecraftMulticastListener) Start() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if !m.stop {
		return fmt.Errorf("already started")
	}

	addr, err := net.ResolveUDPAddr("udp4", m.config.ListenAddress)
	if err != nil {
		return fmt.Errorf("error resolving multicast address: %w", err)
	}

	m.conn, err = listenMulticastUDP("udp4", m.config.Interface, addr)
	if err != nil {
		return fmt.Errorf("error starting multicast listener: %w", err)
	}

	err = m.conn.SetReadBuffer(m.config.BufferSize)
	if err != nil {
		m.conn.Close()
		return fmt.Errorf("error setting multicast read buffer size: %w", err)
	}

	m.stop = false
	m.ctx, m.cancel = context.WithCancel(context.Background())

	go m.listenLoop()

	return nil
}

func (m *MinecraftMulticastListener) listenLoop() {
	var b [maxDatagramSize]byte
	for {
		select {
		case <-m.ctx.Done():
			m.stopped <- struct{}{}
			return
		default:
			rLen, senderAddr, err := m.conn.ReadFromUDP(b[:])
			if err != nil {
				if m.stop {
					m.stopped <- struct{}{}
					return
				}
				log.Printf("Error reading multicast data: %v", err)
				time.Sleep(m.config.RetryDelay)
				continue
			}

			motd, serverPort, err := parseMulticastMessage(b[:rLen])
			if err != nil {
				log.Printf("Error parsing multicast message: %v", err)
				continue
			}

			serverIP := parseAddress(senderAddr)
			m.gameFound(motd, serverIP, serverPort)
		}
	}
}

func (m *MinecraftMulticastListener) Stop() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.stop {
		return fmt.Errorf("already stopped")
	}

	m.stop = true
	m.cancel()
	if m.conn != nil {
		if err := m.conn.Close(); err != nil {
			log.Printf("Error closing connection: %v", err)
		}
	}
	<-m.stopped

	return nil
}
