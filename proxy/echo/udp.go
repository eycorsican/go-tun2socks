package echo

import (
	"net"

	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

// An echo server, do nothing but echo back data to the sender.
type udpHandler struct{}

func NewUDPHandler() core.UDPConnHandler {
	return &udpHandler{}
}

func (h *udpHandler) Connect(conn core.UDPConn, target net.Addr) error {
	return nil
}

func (h *udpHandler) DidReceiveTo(conn core.UDPConn, data []byte, addr net.Addr) error {
	// Dispatch to another goroutine, otherwise will result in deadlock.
	payload := append([]byte(nil), data...)
	go func(b []byte) {
		_, err := conn.WriteFrom(b, addr)
		if err != nil {
			log.Warnf("failed to echo back data: %v", err)
		}
	}(payload)
	return nil
}
