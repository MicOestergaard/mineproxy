package main

import (
	"fmt"
	"github.com/MicOestergaard/mineproxy/multicastlistener"
	"github.com/MicOestergaard/mineproxy/tcpproxy"
	"log"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
)

type gameAnnouncementHandler struct {
	p        *tcpproxy.Proxy
	lock     *sync.Mutex
	lastSeen string
}

func (g *gameAnnouncementHandler) gameFound(serverMOTD, serverIP string, serverPort int) {
	g.lock.Lock()
	defer g.lock.Unlock()

	address := fmt.Sprintf("%s:%d", serverIP, serverPort)
	if address != g.lastSeen {
		log.Println(fmt.Sprintf("Found Minecraft LAN game \"%s\" on %s", serverMOTD, address))
		g.lastSeen = address
		g.p.SetTarget(address)
	}
}

func NewGameAnnouncementHandler(proxy *tcpproxy.Proxy) *gameAnnouncementHandler {
	return &gameAnnouncementHandler{
		p:    proxy,
		lock: new(sync.Mutex),
	}
}

func setupCloseHandler(p *tcpproxy.Proxy, m *multicastlistener.MinecraftMulticastListener) {
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutting down")
		err := m.Stop()
		if err != nil {
			log.Println("Error stopping multicast listener", err)
		}
		err = p.Stop()
		if err != nil {
			log.Println("Error stopping TCP proxy", err)
		}
		os.Exit(0)
	}()
}

func main() {
	tcpListenPort := 25565
	multicastListenAddress := "224.0.2.60:4445"

	if len(os.Args) > 1 {
		var err error
		tcpListenPort, err = strconv.Atoi(os.Args[1])
		if err != nil {
			log.Fatalln("Usage:", os.Args[0], "[port]")
		}
	}

	p := tcpproxy.NewProxy(tcpListenPort)
	g := NewGameAnnouncementHandler(p)
	m := multicastlistener.NewMinecraftMulticastListener(multicastListenAddress, g.gameFound)

	setupCloseHandler(p, m)

	log.Println("Starting TCP proxy on port", tcpListenPort)
	err := p.Start()
	if err != nil {
		log.Fatalln("Error starting TCP proxy:", err)
	}

	log.Println("Starting multicast listener on", multicastListenAddress)
	err = m.Start()
	if err != nil {
		log.Fatalln("Error starting multicast listener:", err)
	}

	select {}
}
