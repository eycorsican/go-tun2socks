package echo

import (
	"net"

	"github.com/eycorsican/go-tun2socks/core"
)

var bufSize = 10 * 1024

type connEntry struct {
	data []byte
	conn core.Connection
}

// An echo proxy, do nothing but echo back data to the sender, the handler was
// created for testing purposes, it may causes issues when more than one clients
// are connecting the handler simultaneously.
type tcpHandler struct {
	buf chan *connEntry
}

func NewTCPHandler() core.ConnectionHandler {
	handler := &tcpHandler{
		buf: make(chan *connEntry, bufSize),
	}
	go handler.echoBack()
	return handler
}

func (h *tcpHandler) echoBack() {
	for {
		e := <-h.buf
		_, err := e.conn.Write(e.data)
		if err != nil {
			e.conn.Close()
		}
	}
}

func (h *tcpHandler) Connect(conn core.Connection, target net.Addr) error {
	return nil
}

func (h *tcpHandler) DidReceive(conn core.Connection, data []byte) error {
	payload := append([]byte(nil), data...)
	// This function runs in lwIP thread, we can't block, so discarding data if
	// buf if full.
	select {
	case h.buf <- &connEntry{data: payload, conn: conn}:
	default:
	}
	return nil
}

func (h *tcpHandler) DidSend(conn core.Connection, len uint16) {
}

func (h *tcpHandler) DidClose(conn core.Connection) {
}

func (h *tcpHandler) LocalDidClose(conn core.Connection) {
}
