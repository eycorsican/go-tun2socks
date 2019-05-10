package echo

import (
	"io"
	"net"

	"github.com/eycorsican/go-tun2socks/core"
)

// An echo proxy, do nothing but echo back data to the sender, the handler was
// created for testing purposes, it may causes issues when more than one clients
// are connecting the handler simultaneously.
type tcpHandler struct{}

func NewTCPHandler() core.TCPConnHandler {
	return &tcpHandler{}
}

func (h *tcpHandler) echoBack(conn net.Conn) {
	io.Copy(conn, conn)
}

func (h *tcpHandler) Handle(conn net.Conn, target net.Addr) error {
	go h.echoBack(conn)
	return nil
}
