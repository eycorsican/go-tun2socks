package main

import (
	"flag"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/eycorsican/go-tun2socks/cmd/tun2socks/v2ray"
	v "github.com/eycorsican/go-tun2socks/proxy/v2ray"
	sscore "github.com/shadowsocks/go-shadowsocks2/core"

	"github.com/eycorsican/go-tun2socks/lwip"
	"github.com/eycorsican/go-tun2socks/proxy/shadowsocks"
	"github.com/eycorsican/go-tun2socks/proxy/socks"
	"github.com/eycorsican/go-tun2socks/tun"
)

const (
	MTU           = 1500
	PROTOCOL_ICMP = 0x1
)

type icmpDelayedWriter struct {
	stack lwip.LWIPStack
	delay int
}

func (w *icmpDelayedWriter) Write(buf []byte) (int, error) {
	if uint8(buf[9]) == PROTOCOL_ICMP && w.delay > 0 {
		payload := make([]byte, len(buf))
		copy(payload, buf)
		go func(data []byte) {
			time.Sleep(time.Duration(w.delay) * time.Millisecond)
			_, err := w.stack.Write(data)
			if err != nil {
				log.Fatal("failed to input data to the stack: %v", err)
			}
		}(payload)
		return len(buf), nil
	} else {
		return w.stack.Write(buf)
	}
}

func main() {
	tunName := flag.String("tunName", "tun1", "TUN interface name")
	tunAddr := flag.String("tunAddr", "240.0.0.2", "TUN interface address")
	tunGw := flag.String("tunGw", "240.0.0.1", "TUN interface gateway")
	tunMask := flag.String("tunMask", "255.255.255.0", "TUN interface netmask")
	dnsServer := flag.String("dnsServer", "114.114.114.114,223.5.5.5", "DNS resolvers for TUN interface. (Only take effect on Windows)")
	proxyType := flag.String("proxyType", "socks", "Proxy handler type: socks, shadowsocks, v2ray")
	vconfig := flag.String("vconfig", "config.json", "Config file for v2ray, in JSON format, and note that routing in v2ray could not violate routes in the routing table")
	sniffingType := flag.String("sniffingType", "http,tls", "Enable domain sniffing for specific kind of traffic in v2ray")
	proxyServer := flag.String("proxyServer", "1.2.3.4:1087", "Proxy server address (host:port) for socks and Shadowsocks proxies")
	proxyCipher := flag.String("proxyCipher", "AEAD_CHACHA20_POLY1305", "Cipher used for Shadowsocks proxy, available ciphers: "+strings.Join(sscore.ListCipher(), " "))
	proxyPassword := flag.String("proxyPassword", "", "Password used for Shadowsocks proxy")
	delayICMP := flag.Int("delayICMP", 10, "Delay ICMP packets for a short period of time, in milliseconds")
	udpTimeout := flag.Duration("udpTimeout", 1*time.Minute, "Set timeout for UDP proxy connections in socks and Shadowsocks")

	flag.Parse()

	parts := strings.Split(*proxyServer, ":")
	if len(parts) != 2 {
		log.Fatal("invalid server address")
	}
	proxyAddr := parts[0]
	port, err := strconv.Atoi(parts[1])
	if err != nil {
		log.Fatal("invalid server port")
	}
	proxyPort := uint16(port)

	// Open the tun device.
	dnsServers := strings.Split(*dnsServer, ",")
	tunDev, err := tun.OpenTunDevice(*tunName, *tunAddr, *tunGw, *tunMask, dnsServers)
	if err != nil {
		log.Fatalf("failed to open tun device: %v", err)
	}

	// Setup TCP/IP stack.
	lwipStack := lwip.NewLWIPStack()
	lwipWriter := &icmpDelayedWriter{stack: lwipStack, delay: *delayICMP}

	// Register TCP and UDP handlers to handle accepted connections.
	switch *proxyType {
	case "socks":
		lwip.RegisterTCPConnectionHandler(socks.NewTCPHandler(proxyAddr, proxyPort))
		lwip.RegisterUDPConnectionHandler(socks.NewUDPHandler(proxyAddr, proxyPort, *udpTimeout))
		break
	case "shadowsocks":
		if *proxyCipher == "" || *proxyPassword == "" {
			log.Fatal("invalid cipher or password")
		}
		log.Printf("creat Shadowsocks handler: %v:%v@%v:%v", *proxyCipher, *proxyPassword, proxyAddr, proxyPort)
		lwip.RegisterTCPConnectionHandler(shadowsocks.NewTCPHandler(net.JoinHostPort(proxyAddr, strconv.Itoa(int(proxyPort))), *proxyCipher, *proxyPassword))
		lwip.RegisterUDPConnectionHandler(shadowsocks.NewUDPHandler(net.JoinHostPort(proxyAddr, strconv.Itoa(int(proxyPort))), *proxyCipher, *proxyPassword, *udpTimeout))
		break
	case "v2ray":
		configBytes, err := ioutil.ReadFile(*vconfig)
		if err != nil {
			log.Fatal("invalid vconfig file")
		}
		var validSniffings []string
		sniffings := strings.Split(*sniffingType, ",")
		for _, s := range sniffings {
			if s == "http" || s == "tls" {
				validSniffings = append(validSniffings, s)
			}
		}
		vhandler := v.NewHandler("json", configBytes, validSniffings)
		lwip.RegisterTCPConnectionHandler(vhandler)
		lwip.RegisterUDPConnectionHandler(vhandler)
		break
	default:
		log.Fatal("unsupported proxy type")
	}

	// Register an output function to write packets output from lwip stack to tun
	// device, output function should be set before input any packets.
	lwip.RegisterOutputFn(func(data []byte) (int, error) {
		return tunDev.Write(data)
	})

	// Copy packets from tun device to lwip stack.
	go func() {
		_, err := io.CopyBuffer(lwipWriter, tunDev, make([]byte, MTU))
		if err != nil {
			log.Fatal("copying data failed: %v", err)
		}
	}()

	log.Printf("running tun2socks")

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGHUP)
	<-osSignals
}
