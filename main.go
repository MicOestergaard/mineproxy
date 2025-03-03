package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/MicOestergaard/mineproxy/multicastlistener"
	"github.com/MicOestergaard/mineproxy/tcpproxy"
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
		log.Printf("Found Minecraft LAN game \"%s\" on %s", serverMOTD, address)
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

func promptForServer() (string, int, error) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter server IP address: ")
	ip, err := reader.ReadString('\n')
	if err != nil {
		return "", 0, fmt.Errorf("error reading IP address: %w", err)
	}
	ip = strings.TrimSpace(ip)
	if ip == "" {
		ip = "127.0.0.1"
	}

	fmt.Print("Enter server port: ")
	portStr, err := reader.ReadString('\n')
	if err != nil {
		return "", 0, fmt.Errorf("error reading port: %w", err)
	}
	portStr = strings.TrimSpace(portStr)

	var port int
	if portStr != "" {
		port, err = strconv.Atoi(portStr)
		if err != nil {
			return "", 0, fmt.Errorf("invalid port number: %w", err)
		}
		if port <= 0 || port > 65535 {
			return "", 0, fmt.Errorf("port number must be between 1 and 65535")
		}
	} else {
		return "", 0, fmt.Errorf("port cannot be empty")
	}

	return ip, port, nil
}

func setupCloseHandler(p *tcpproxy.Proxy, m *multicastlistener.MinecraftMulticastListener) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		log.Println("Shutting down gracefully...")

		// Stop multicast listener if it's running
		if m != nil {
			if err := m.Stop(); err != nil {
				log.Printf("Error stopping multicast listener: %v", err)
			}
		}

		// Stop the proxy
		if err := p.Stop(); err != nil {
			log.Printf("Error stopping TCP proxy: %v", err)
		}

		log.Println("Shutdown complete")
		os.Exit(0)
	}()
}

func main() {
	tcpListenPort := 25565
	multicastAddress := "224.0.2.60:4445"

	if len(os.Args) > 1 {
		var err error
		tcpListenPort, err = strconv.Atoi(os.Args[1])
		if err != nil {
			log.Fatalln("Usage:", os.Args[0], "[port]")
		}
	}

	fmt.Println("Choose mode:")
	fmt.Println("1. Automatic discovery (LAN)")
	fmt.Println("2. Manual server input")
	fmt.Print("Enter choice (1 or 2): ")

	reader := bufio.NewReader(os.Stdin)
	choice, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Error reading input: %v", err)
	}
	choice = strings.TrimSpace(choice)

	p := tcpproxy.NewProxy(tcpListenPort)
	var m *multicastlistener.MinecraftMulticastListener

	switch choice {
	case "1":
		// Automatic discovery mode

		config := multicastlistener.Config{
			ListenAddress: multicastAddress,
			BufferSize:    1024,
			RetryDelay:    time.Second,
			Interface:     nil, // Use default interface
		}

		g := NewGameAnnouncementHandler(p)
		m = multicastlistener.NewMinecraftMulticastListener(config, g.gameFound)

		log.Printf("Starting multicast listener on %s", multicastAddress)
		if err := m.Start(); err != nil {
			log.Fatalf("Error starting multicast listener: %v", err)
		}

	case "2":
		// Manual input mode

		serverIP, serverPort, err := promptForServer()
		if err != nil {
			log.Fatalf("Error getting server details: %v", err)
		}

		address := fmt.Sprintf("%s:%d", serverIP, serverPort)
		log.Printf("Using server address: %s", address)
		p.SetTarget(address)

	default:
		log.Fatalln("Invalid choice. Please enter 1 or 2.")
	}

	setupCloseHandler(p, m)

	log.Printf("Starting TCP proxy on port %d", tcpListenPort)
	if err := p.Start(); err != nil {
		log.Fatalf("Error starting TCP proxy: %v", err)
	}

	select {}
}
