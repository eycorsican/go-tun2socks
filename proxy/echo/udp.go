package echo

import (
	"log"
	"net"

	"github.com/eycorsican/go-tun2socks/core"
)

// An echo server, do nothing but echo back data to the sender.
type udpHandler struct{}

func NewUDPHandler() core.ConnectionHandler {
	return &udpHandler{}
}

func (h *udpHandler) Connect(conn core.Connection, target net.Addr) error {
	return nil
}

func (h *udpHandler) DidReceive(conn core.Connection, data []byte) error {
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

func (h *udpHandler) DidSend(conn core.Connection, len uint16) {
}

func (h *udpHandler) DidClose(conn core.Connection) {
}

func (h *udpHandler) LocalDidClose(conn core.Connection) {
}
