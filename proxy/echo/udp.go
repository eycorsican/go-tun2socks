package echo

import (
	"log"
	"net"

	tun2socks "github.com/eycorsican/go-tun2socks"
)

// An echo server, do nothing but echo back data to the sender.
type udpHandler struct{}

func NewUDPHandler() tun2socks.ConnectionHandler {
	return &udpHandler{}
}

func (h *udpHandler) Connect(conn tun2socks.Connection, target net.Addr) error {
	return nil
}

func (h *udpHandler) DidReceive(conn tun2socks.Connection, data []byte) error {
	// Dispatch to another goroutine, otherwise will result in deadlock.
	payload := append([]byte(nil), data...)
	go func(b []byte) {
		_, err := conn.Write(b)
		if err != nil {
			log.Printf("failed to echo back data: %v", err)
		}
	}(payload)
	return nil
}

func (h *udpHandler) DidSend(conn tun2socks.Connection, len uint16) {
}

func (h *udpHandler) DidClose(conn tun2socks.Connection) {
}

func (h *udpHandler) DidAbort(conn tun2socks.Connection) {
}

func (h *udpHandler) DidReset(conn tun2socks.Connection) {
}

func (h *udpHandler) LocalDidClose(conn tun2socks.Connection) {
}
