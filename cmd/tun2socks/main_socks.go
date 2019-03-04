// +build socks

package main

import (
	"flag"
	"log"
	"net"
	"time"

	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy/socks"
)

func init() {
	args.ProxyServer = flag.String("proxyServer", "1.2.3.4:1087", "Proxy server address (host:port) for socks and Shadowsocks proxies")
	args.UdpTimeout = flag.Duration("udpTimeout", 1*time.Minute, "Set timeout for UDP proxy connections in SOCKS and Shadowsocks")
	args.Applog = flag.Bool("applog", false, "Enable app logging (V2Ray, Shadowsocks and SOCKS5 handler)")

	registerHandlerCreater("socks", func() {
		// Verify proxy server address.
		proxyAddr, err := net.ResolveTCPAddr("tcp", *args.ProxyServer)
		if err != nil {
			log.Fatalf("invalid proxy server address: %v", err)
		}
		proxyHost := proxyAddr.IP.String()
		proxyPort := uint16(proxyAddr.Port)

		core.RegisterTCPConnectionHandler(socks.NewTCPHandler(proxyHost, proxyPort))
		core.RegisterUDPConnectionHandler(socks.NewUDPHandler(proxyHost, proxyPort, *args.UdpTimeout, dnsCache))
	})
}
