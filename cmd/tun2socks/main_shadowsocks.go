// +build shadowsocks

package main

import (
	"flag"
	"log"
	"net"
	"strings"
	"time"

	sscore "github.com/shadowsocks/go-shadowsocks2/core"

	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy/shadowsocks"
)

func init() {
	args.ProxyServer = flag.String("proxyServer", "1.2.3.4:1087", "Proxy server address (host:port) for socks and Shadowsocks proxies")
	args.ProxyCipher = flag.String("proxyCipher", "AEAD_CHACHA20_POLY1305", "Cipher used for Shadowsocks proxy, available ciphers: "+strings.Join(sscore.ListCipher(), " "))
	args.ProxyPassword = flag.String("proxyPassword", "", "Password used for Shadowsocks proxy")
	args.Applog = flag.Bool("applog", false, "Enable app logging (V2Ray, Shadowsocks and SOCKS5 handler)")
	args.UdpTimeout = flag.Duration("udpTimeout", 1*time.Minute, "Set timeout for UDP proxy connections in SOCKS and Shadowsocks")

	registerHandlerCreater("shadowsocks", func() {
		// Verify proxy server address.
		proxyAddr, err := net.ResolveTCPAddr("tcp", *args.ProxyServer)
		if err != nil {
			log.Fatalf("invalid proxy server address: %v", err)
		}
		proxyHost := proxyAddr.IP.String()
		proxyPort := uint16(proxyAddr.Port)

		if *args.ProxyCipher == "" || *args.ProxyPassword == "" {
			log.Fatal("invalid cipher or password")
		}
		core.RegisterTCPConnectionHandler(shadowsocks.NewTCPHandler(core.ParseTCPAddr(proxyHost, proxyPort).String(), *args.ProxyCipher, *args.ProxyPassword))
		core.RegisterUDPConnectionHandler(shadowsocks.NewUDPHandler(core.ParseUDPAddr(proxyHost, proxyPort).String(), *args.ProxyCipher, *args.ProxyPassword, *args.UdpTimeout, dnsCache))
	})
}
