package echo

import (
	"log"
	"net"

	tun2socks "github.com/eycorsican/go-tun2socks"
)

type connEntry struct {
	data []byte
	conn tun2socks.Connection
}

// An echo server, do nothing but echo back data to the sender.
type tcpHandler struct {
	buf chan *connEntry
}

func NewTCPHandler() tun2socks.ConnectionHandler {
	handler := &tcpHandler{
		buf: make(chan *connEntry, 1024),
	}
	go handler.echoBack()
	return handler
}

func (h *tcpHandler) echoBack() {
	for {
		e := <-h.buf
		_, err := e.conn.Write(e.data)
		if err != nil {
			log.Printf("failed to echo back data: %v", err)
			e.conn.Close()
		}
	}
}

func (h *tcpHandler) Connect(conn tun2socks.Connection, target net.Addr) error {
	return nil
}

func (h *tcpHandler) DidReceive(conn tun2socks.Connection, data []byte) error {
	payload := append([]byte(nil), data...)
	h.buf <- &connEntry{data: payload, conn: conn}
	return nil
}

func (h *tcpHandler) DidSend(conn tun2socks.Connection, len uint16) {
}

func (h *tcpHandler) DidClose(conn tun2socks.Connection) {
}

func (h *tcpHandler) LocalDidClose(conn tun2socks.Connection) {
}
