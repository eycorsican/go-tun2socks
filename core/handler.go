package core

import (
	"net"
)

// TCPConnHandler handles TCP connections comming from TUN.
type TCPConnHandler interface {
	// Handle handles the conn for target.
	Handle(conn net.Conn, target net.Addr) error
}

// UDPConnHandler handles UDP connections comming from TUN.
type UDPConnHandler interface {
	// Connect connects the proxy server. `target` can be nil.
	Connect(conn UDPConn, target net.Addr) error

	// DidReceive will be called when data arrives from TUN.
	DidReceiveTo(conn UDPConn, data []byte, addr net.Addr) error
}

var tcpConnHandler TCPConnHandler
var udpConnHandler UDPConnHandler

func RegisterTCPConnHandler(h TCPConnHandler) {
	tcpConnHandler = h
}

func RegisterUDPConnHandler(h UDPConnHandler) {
	udpConnHandler = h
}
