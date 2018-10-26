package echo

import (
	"log"
	"net"

	"github.com/eycorsican/go-tun2socks/lwip"
)

// An echo server, do nothing but echo back data to the sender.
type udpHandler struct{}

func NewUDPHandler() lwip.ConnectionHandler {
	return &udpHandler{}
}

func (h *udpHandler) Connect(conn lwip.Connection, target net.Addr) error {
	return nil
}

func (h *udpHandler) DidReceive(conn lwip.Connection, data []byte) error {
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

func (h *udpHandler) DidSend(conn lwip.Connection, len uint16) {
}

func (h *udpHandler) DidClose(conn lwip.Connection) {
}

func (h *udpHandler) LocalDidClose(conn lwip.Connection) {
}
