// +build redirect

package main

import (
	"flag"
	"time"

	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy/redirect"
)

func init() {
	args.ProxyServer = flag.String("proxyServer", "1.2.3.4:1087", "Proxy server address (host:port) for socks and Shadowsocks proxies")
	args.UdpTimeout = flag.Duration("udpTimeout", 1*time.Minute, "Set timeout for UDP proxy connections in SOCKS and Shadowsocks")

	registerHandlerCreater("redirect", func() {
		core.RegisterTCPConnectionHandler(redirect.NewTCPHandler(*args.ProxyServer))
		core.RegisterUDPConnectionHandler(redirect.NewUDPHandler(*args.ProxyServer, *args.UdpTimeout))
	})
}
