package main

import (
	"flag"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	sscore "github.com/shadowsocks/go-shadowsocks2/core"

	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/filter"
	"github.com/eycorsican/go-tun2socks/proxy"
	"github.com/eycorsican/go-tun2socks/proxy/echo"
	"github.com/eycorsican/go-tun2socks/proxy/redirect"
	"github.com/eycorsican/go-tun2socks/proxy/shadowsocks"
	"github.com/eycorsican/go-tun2socks/proxy/socks"
	"github.com/eycorsican/go-tun2socks/tun"
)

const (
	MTU = 1500
)

func main() {
	tunName := flag.String("tunName", "tun1", "TUN interface name")
	tunAddr := flag.String("tunAddr", "240.0.0.2", "TUN interface address")
	tunGw := flag.String("tunGw", "240.0.0.1", "TUN interface gateway")
	tunMask := flag.String("tunMask", "255.255.255.0", "TUN interface netmask, as for IPv6, it's the prefixlen")
	dnsServer := flag.String("dnsServer", "114.114.114.114,223.5.5.5", "DNS resolvers for TUN interface (only take effect on Windows)")
	proxyType := flag.String("proxyType", "socks", "Proxy handler type: socks, shadowsocks")
	proxyServer := flag.String("proxyServer", "1.2.3.4:1087", "Proxy server address (host:port) for socks and Shadowsocks proxies")
	proxyCipher := flag.String("proxyCipher", "AEAD_CHACHA20_POLY1305", "Cipher used for Shadowsocks proxy, available ciphers: "+strings.Join(sscore.ListCipher(), " "))
	proxyPassword := flag.String("proxyPassword", "", "Password used for Shadowsocks proxy")
	delayICMP := flag.Int("delayICMP", 10, "Delay ICMP packets for a short period of time, in milliseconds")
	udpTimeout := flag.Duration("udpTimeout", 1*time.Minute, "Set timeout for UDP proxy connections in socks and Shadowsocks")
	applog := flag.Bool("applog", false, "Enable app logging (V2Ray and SOCKS5 handler)")
	disableDNSCache := flag.Bool("disableDNSCache", false, "Disable DNS cache (SOCKS5 and Shadowsocks handler)")

	flag.Parse()

	// Verify proxy server address.
	proxyAddr, err := net.ResolveTCPAddr("tcp", *proxyServer)
	if err != nil {
		log.Fatalf("invalid proxy server address: %v", err)
	}
	proxyHost := proxyAddr.IP.String()
	proxyPort := uint16(proxyAddr.Port)

	// Open the tun device.
	dnsServers := strings.Split(*dnsServer, ",")
	tunDev, err := tun.OpenTunDevice(*tunName, *tunAddr, *tunGw, *tunMask, dnsServers)
	if err != nil {
		log.Fatalf("failed to open tun device: %v", err)
	}

	// Setup TCP/IP stack.
	lwipWriter := core.NewLWIPStack().(io.Writer)

	// Wrap a writer to delay ICMP packets if delay time is not zero.
	if *delayICMP > 0 {
		log.Printf("ICMP packets will be delayed for %dms", *delayICMP)
		lwipWriter = filter.NewICMPFilter(lwipWriter, *delayICMP).(io.Writer)
	}

	// Wrap a writer to print out processes the creating network connections.
	if *applog {
		log.Printf("App logging is enabled")
		lwipWriter = filter.NewApplogFilter(lwipWriter).(io.Writer)
	}

	// Register TCP and UDP handlers to handle accepted connections.
	switch *proxyType {
	case "echo":
		core.RegisterTCPConnectionHandler(echo.NewTCPHandler())
		core.RegisterUDPConnectionHandler(echo.NewUDPHandler())
		break
	case "redirect":
		core.RegisterTCPConnectionHandler(redirect.NewTCPHandler(*proxyServer))
		core.RegisterUDPConnectionHandler(redirect.NewUDPHandler(*proxyServer, *udpTimeout))
		break
	case "socks":
		core.RegisterTCPConnectionHandler(socks.NewTCPHandler(proxyHost, proxyPort))
		if *disableDNSCache {
			core.RegisterUDPConnectionHandler(socks.NewUDPHandler(proxyHost, proxyPort, *udpTimeout, nil))
		} else {
			core.RegisterUDPConnectionHandler(socks.NewUDPHandler(proxyHost, proxyPort, *udpTimeout, proxy.NewDNSCache()))
		}
		break
	case "shadowsocks":
		if *proxyCipher == "" || *proxyPassword == "" {
			log.Fatal("invalid cipher or password")
		}
		core.RegisterTCPConnectionHandler(shadowsocks.NewTCPHandler(core.ParseTCPAddr(proxyHost, proxyPort).String(), *proxyCipher, *proxyPassword))
		if *disableDNSCache {
			core.RegisterUDPConnectionHandler(shadowsocks.NewUDPHandler(core.ParseUDPAddr(proxyHost, proxyPort).String(), *proxyCipher, *proxyPassword, *udpTimeout, nil))
		} else {
			core.RegisterUDPConnectionHandler(shadowsocks.NewUDPHandler(core.ParseUDPAddr(proxyHost, proxyPort).String(), *proxyCipher, *proxyPassword, *udpTimeout, proxy.NewDNSCache()))
		}
		break
	default:
		log.Fatal("unsupported proxy type")
	}

	// Register an output callback to write packets output from lwip stack to tun
	// device, output function should be set before input any packets.
	core.RegisterOutputFn(func(data []byte) (int, error) {
		return tunDev.Write(data)
	})

	// Copy packets from tun device to lwip stack, it's the main loop.
	go func() {
		_, err := io.CopyBuffer(lwipWriter, tunDev, make([]byte, MTU))
		if err != nil {
			log.Fatalf("copying data failed: %v", err)
		}
	}()

	log.Printf("Running tun2socks")

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGHUP)
	<-osSignals
}
