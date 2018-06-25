package echo

import (
	"log"
	"net"

	tun2socks "github.com/eycorsican/go-tun2socks"
)

// An echo server, do nothing but echo back data to the sender.
type tcpHandler struct{}

func NewTCPHandler() tun2socks.ConnectionHandler {
	return &tcpHandler{}
}

func (h *tcpHandler) Connect(conn tun2socks.Connection, target net.Addr) {
}

func (h *tcpHandler) DidReceive(conn tun2socks.Connection, data []byte) error {
	// Dispatch to another goroutine, otherwise will result in deadlock.
	payload := append([]byte(nil), data...)
	go func(b []byte) {
		err := conn.Write(b)
		if err != nil {
			log.Printf("failed to echo back data: %v", err)
		}
	}(payload)
	return nil
}

func (h *tcpHandler) DidSend(conn tun2socks.Connection, len uint16) {
}

func (h *tcpHandler) DidClose(conn tun2socks.Connection) {
}

func (h *tcpHandler) DidAbort(conn tun2socks.Connection) {
}

func (h *tcpHandler) DidReset(conn tun2socks.Connection) {
}

func (h *tcpHandler) LocalDidClose(conn tun2socks.Connection) {
}
