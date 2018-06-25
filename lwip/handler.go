package lwip

import (
	tun2socks "github.com/eycorsican/go-tun2socks"
)

var tcpConnectionHandler tun2socks.ConnectionHandler
var udpConnectionHandler tun2socks.ConnectionHandler

func RegisterTCPConnectionHandler(h tun2socks.ConnectionHandler) {
	tcpConnectionHandler = h
}

func RegisterUDPConnectionHandler(h tun2socks.ConnectionHandler) {
	udpConnectionHandler = h
}
