package socks

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"github.com/eycorsican/go-tun2socks/lwip"
)

type tcpHandler struct {
	sync.Mutex

	proxyHost string
	proxyPort uint16
	conns     map[lwip.Connection]net.Conn
}

func NewTCPHandler(proxyHost string, proxyPort uint16) lwip.ConnectionHandler {
	return &tcpHandler{
		proxyHost: proxyHost,
		proxyPort: proxyPort,
		conns:     make(map[lwip.Connection]net.Conn, 16),
	}
}

func (h *tcpHandler) fetchInput(conn lwip.Connection, input io.Reader) {
	defer func() {
		h.Close(conn)
		conn.Close() // also close tun2socks connection here
	}()

	_, err := io.Copy(conn, input)
	if err != nil {
		log.Printf("fetch input failed: %v", err)
	}
}

func (h *tcpHandler) getConn(conn lwip.Connection) (net.Conn, bool) {
	h.Lock()
	defer h.Unlock()
	if c, ok := h.conns[conn]; ok {
		return c, true
	}
	return nil, false
}

func (h *tcpHandler) Connect(conn lwip.Connection, target net.Addr) error {
	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%d", h.proxyHost, h.proxyPort), nil, nil)
	if err != nil {
		return err
	}
	c, err := dialer.Dial(target.Network(), target.String())
	if err != nil {
		return err
	}
	h.Lock()
	h.conns[conn] = c
	h.Unlock()
	c.SetDeadline(time.Time{})
	go h.fetchInput(conn, c)
	return nil
}

func (h *tcpHandler) DidReceive(conn lwip.Connection, data []byte) error {
	if c, found := h.getConn(conn); found {
		_, err := c.Write(data)
		if err != nil {
			h.Close(conn)
			return errors.New(fmt.Sprintf("write remote failed: %v", err))
		}
		return nil
	} else {
		return errors.New(fmt.Sprintf("proxy connection %v->%v does not exists", conn.LocalAddr(), conn.RemoteAddr()))
	}
}

func (h *tcpHandler) DidSend(conn lwip.Connection, len uint16) {
}

func (h *tcpHandler) DidClose(conn lwip.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) LocalDidClose(conn lwip.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) Close(conn lwip.Connection) {
	if c, found := h.getConn(conn); found {
		c.Close()
		h.Lock()
		delete(h.conns, conn)
		h.Unlock()
	}
}
