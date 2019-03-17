package core

import (
	"net"
)

// TCPConnHandler handles TCP connections comming from TUN.
type TCPConnHandler interface {
	// Connect connects the proxy server.
	Connect(conn TCPConn, target net.Addr) error

	// DidReceive will be called when data arrives from TUN.
	DidReceive(conn TCPConn, data []byte) error

	// DidClose will be called when the connection has been closed.
	DidClose(conn TCPConn)

	// LocalDidClose will be called when local client has close the connection.
	LocalDidClose(conn TCPConn)
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
