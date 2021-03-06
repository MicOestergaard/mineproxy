package multicastlistener

import (
	"fmt"
	"golang.org/x/net/ipv4"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	maxDatagramSize = 1024
)

var regex = regexp.MustCompile(`\[MOTD]([^\[\]]*)\[/MOTD].*\[AD]([^\[\]]*)\[/AD]`)

type MinecraftMulticastListener struct {
	listenAddress string
	conn          *net.UDPConn
	lock          *sync.Mutex
	stop          bool
	stopped       chan struct{}
	gameFound     func(motd, serverIP string, serverPort int)
}

func NewMinecraftMulticastListener(listenAddress string, gameFound func(serverMOTD, serverIP string, serverPort int)) *MinecraftMulticastListener {
	return &MinecraftMulticastListener{
		listenAddress: listenAddress,
		lock:          new(sync.Mutex),
		stop:          true,
		stopped:       make(chan struct{}),
		gameFound:     gameFound,
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

	if !m.stop {
		m.lock.Unlock()
		return fmt.Errorf("already started")
	}

	addr, err := net.ResolveUDPAddr("udp4", m.listenAddress)
	if err != nil {
		m.lock.Unlock()
		return fmt.Errorf("error resolving multicast address: %s", err)
	}

	m.conn, err = listenMulticastUDP("udp4", nil, addr)
	if err != nil {
		m.lock.Unlock()
		return fmt.Errorf("error starting multicast listener: %s", err)
	}

	err = m.conn.SetReadBuffer(maxDatagramSize)
	if err != nil {
		m.lock.Unlock()
		return fmt.Errorf("error setting multicast read buffer size: %s", err)
	}

	m.stop = false
	m.lock.Unlock()

	go func() {
		var b [maxDatagramSize]byte
		for {
			rLen, senderAddr, err := m.conn.ReadFromUDP(b[:])
			if err != nil {
				if m.stop {
					m.stopped <- struct{}{}
					break
				}
				log.Println("Error reading multicast data:", err)
				time.Sleep(1 * time.Second)
				continue
			}

			motd, serverPort, err := parseMulticastMessage(b[:rLen])
			if err != nil {
				log.Println("Error parsing multicast message:", err)
				continue
			}

			serverIP := parseAddress(senderAddr)
			m.gameFound(motd, serverIP, serverPort)
		}
	}()

	return nil
}

func (m *MinecraftMulticastListener) Stop() error {
	m.lock.Lock()
	defer m.lock.Unlock()

	if m.stop {
		return fmt.Errorf("already stopped")
	}

	m.stop = true
	_ = m.conn.Close()
	<-m.stopped

	return nil
}
