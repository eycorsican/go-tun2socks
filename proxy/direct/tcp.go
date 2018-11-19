package direct

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct {
	sync.Mutex
	conns map[core.Connection]net.Conn
}

func NewTCPHandler() core.ConnectionHandler {
	return &tcpHandler{
		conns: make(map[core.Connection]net.Conn, 16),
	}
}

func (h *tcpHandler) fetchInput(conn core.Connection, input io.Reader) {
	_, err := io.Copy(conn, input)
	if err != nil {
		h.Close(conn)
		conn.Close()
	}
}

func (h *tcpHandler) Connect(conn core.Connection, target net.Addr) error {
	c, err := net.Dial("tcp", target.String())
	if err != nil {
		return err
	}
	h.Lock()
	h.conns[conn] = c
	h.Unlock()
	c.SetReadDeadline(time.Time{})
	go h.fetchInput(conn, c)
	log.Printf("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}

func (h *tcpHandler) DidReceive(conn core.Connection, data []byte) error {
	h.Lock()
	c, ok := h.conns[conn]
	h.Unlock()
	if ok {
		_, err := c.Write(data)
		if err != nil {
			h.Close(conn)
			return errors.New(fmt.Sprintf("write remote failed: %v", err))
		}
		return nil
	} else {
		h.Close(conn)
		return errors.New(fmt.Sprintf("proxy connection %v->%v does not exists", conn.LocalAddr(), conn.RemoteAddr()))
	}
}

func (h *tcpHandler) DidSend(conn core.Connection, len uint16) {
	// unused
}

func (h *tcpHandler) DidClose(conn core.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) LocalDidClose(conn core.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) Close(conn core.Connection) {
	h.Lock()
	defer h.Unlock()

	if c, found := h.conns[conn]; found {
		c.Close()
	}
	delete(h.conns, conn)
}
