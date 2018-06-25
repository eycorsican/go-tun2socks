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

	tun2socks "github.com/eycorsican/go-tun2socks"
	"github.com/eycorsican/go-tun2socks/lwip"
)

type tcpHandler struct {
	sync.Mutex

	proxyHost string
	proxyPort uint16
	conns     map[tun2socks.Connection]net.Conn
}

func NewTCPHandler(proxyHost string, proxyPort uint16) tun2socks.ConnectionHandler {
	return &tcpHandler{
		proxyHost: proxyHost,
		proxyPort: proxyPort,
		conns:     make(map[tun2socks.Connection]net.Conn, 16),
	}
}

func (h *tcpHandler) fetchInput(conn tun2socks.Connection, input io.Reader) {
	buf := lwip.NewBytes()

	defer func() {
		h.Close(conn)
		lwip.FreeBytes(buf)
	}()

	for {
		n, err := input.Read(buf)
		if err != nil {
			if err != io.EOF {
				log.Printf("failed to read from SOCKS5 server: %v", err)
				h.Close(conn)
				return
			}
			break
		}
		// No copy, since we are using TCP_WRITE_FLAG_COPY in tcp_write()
		err = conn.Write(buf[:n])
		if err != nil {
			log.Printf("failed to send data to TUN: %v", err)
			h.Close(conn)
			return
		}
	}
}

func (h *tcpHandler) getConn(conn tun2socks.Connection) (net.Conn, bool) {
	h.Lock()
	defer h.Unlock()
	if c, ok := h.conns[conn]; ok {
		return c, true
	}
	return nil, false
}

func (h *tcpHandler) Connect(conn tun2socks.Connection, target net.Addr) {
	dialer, err := proxy.SOCKS5("tcp", fmt.Sprintf("%s:%d", h.proxyHost, h.proxyPort), nil, nil)
	if err != nil {
		log.Printf("failed to create SOCKS5 dialer: %v", err)
		return
	}
	c, err := dialer.Dial(target.Network(), target.String())
	if err != nil {
		log.Printf("failed to dial SOCKS5 server: %v", err)
		return
	}
	h.Lock()
	h.conns[conn] = c
	h.Unlock()
	c.SetDeadline(time.Time{})
	go h.fetchInput(conn, c)
}

func (h *tcpHandler) DidReceive(conn tun2socks.Connection, data []byte) error {
	if c, found := h.getConn(conn); found {
		_, err := c.Write(data)
		if err != nil {
			log.Printf("failed to write data to SOCKS5 server: %v", err)
			h.Close(conn)
			return errors.New("failed to write data")
		}
		return nil
	} else {
		return errors.New(fmt.Sprintf("proxy connection does not exists: %v <-> %v", conn.LocalAddr().String(), conn.RemoteAddr().String()))
	}
}

func (h *tcpHandler) DidSend(conn tun2socks.Connection, len uint16) {
}

func (h *tcpHandler) DidClose(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) DidAbort(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) DidReset(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) LocalDidClose(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) Close(conn tun2socks.Connection) {
	if c, found := h.getConn(conn); found {
		c.Close()
		h.Lock()
		delete(h.conns, conn)
		h.Unlock()
	}
}
