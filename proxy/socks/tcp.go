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

	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct {
	sync.Mutex

	proxyHost string
	proxyPort uint16
	conns     map[core.Connection]net.Conn
}

func NewTCPHandler(proxyHost string, proxyPort uint16) core.ConnectionHandler {
	return &tcpHandler{
		proxyHost: proxyHost,
		proxyPort: proxyPort,
		conns:     make(map[core.Connection]net.Conn, 16),
	}
}

func (h *tcpHandler) fetchInput(conn core.Connection, input io.Reader) {
	// FIXME maybe use a larger buffer?
	buf := core.NewBytes(core.BufSize) // 2k buf

	defer func() {
		h.Close(conn)
		conn.Close() // also close tun2socks connection here
		core.FreeBytes(buf)
	}()

	_, err := io.CopyBuffer(conn, input, buf)
	if err != nil {
		// log.Printf("fetch input failed: %v", err)
		return
	}
}

func (h *tcpHandler) getConn(conn core.Connection) (net.Conn, bool) {
	h.Lock()
	defer h.Unlock()
	if c, ok := h.conns[conn]; ok {
		return c, true
	}
	return nil, false
}

func (h *tcpHandler) Connect(conn core.Connection, target net.Addr) error {
	dialer, err := proxy.SOCKS5("tcp", core.ParseTCPAddr(h.proxyHost, h.proxyPort).String(), nil, nil)
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
	log.Printf("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}

func (h *tcpHandler) DidReceive(conn core.Connection, data []byte) error {
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

func (h *tcpHandler) DidSend(conn core.Connection, len uint16) {
}

func (h *tcpHandler) DidClose(conn core.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) LocalDidClose(conn core.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) Close(conn core.Connection) {
	if c, found := h.getConn(conn); found {
		c.Close()
		h.Lock()
		delete(h.conns, conn)
		h.Unlock()
	}
}
