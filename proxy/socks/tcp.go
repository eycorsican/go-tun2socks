package socks

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"golang.org/x/net/proxy"

	"github.com/eycorsican/go-tun2socks/common/dns"
	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct {
	sync.Mutex

	proxyHost string
	proxyPort uint16
	conns     map[core.TCPConn]net.Conn

	fakeDns dns.FakeDns
}

func NewTCPHandler(proxyHost string, proxyPort uint16, fakeDns dns.FakeDns) core.TCPConnHandler {
	return &tcpHandler{
		proxyHost: proxyHost,
		proxyPort: proxyPort,
		conns:     make(map[core.TCPConn]net.Conn, 16),
		fakeDns:   fakeDns,
	}
}

func (h *tcpHandler) fetchInput(conn core.TCPConn, input io.Reader) {
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

func (h *tcpHandler) getConn(conn core.TCPConn) (net.Conn, bool) {
	h.Lock()
	defer h.Unlock()
	if c, ok := h.conns[conn]; ok {
		return c, true
	}
	return nil, false
}

func (h *tcpHandler) Connect(conn core.TCPConn, target net.Addr) error {
	dialer, err := proxy.SOCKS5("tcp", core.ParseTCPAddr(h.proxyHost, h.proxyPort).String(), nil, nil)
	if err != nil {
		return err
	}

	// Replace with a domain name if target address IP is a fake IP.
	host, port, err := net.SplitHostPort(target.String())
	if err != nil {
		log.Errorf("error when split host port %v", err)
	}
	var targetHost string = host
	if h.fakeDns != nil {
		if ip := net.ParseIP(host); ip != nil {
			if h.fakeDns.IsFakeIP(ip) {
				targetHost = h.fakeDns.QueryDomain(ip)
			}
		}
	}
	dest := fmt.Sprintf("%s:%s", targetHost, port)

	c, err := dialer.Dial(target.Network(), dest)
	if err != nil {
		return err
	}
	h.Lock()
	h.conns[conn] = c
	h.Unlock()
	c.SetDeadline(time.Time{})
	go h.fetchInput(conn, c)
	log.Infof("new proxy connection for target: %s:%s", target.Network(), fmt.Sprintf("%s:%s", targetHost, port))
	return nil
}

func (h *tcpHandler) DidReceive(conn core.TCPConn, data []byte) error {
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

func (h *tcpHandler) DidSend(conn core.TCPConn, len uint16) {
}

func (h *tcpHandler) DidClose(conn core.TCPConn) {
	h.Close(conn)
}

func (h *tcpHandler) LocalDidClose(conn core.TCPConn) {
	h.Close(conn)
}

func (h *tcpHandler) Close(conn core.TCPConn) {
	if c, found := h.getConn(conn); found {
		c.Close()
		h.Lock()
		delete(h.conns, conn)
		h.Unlock()
	}
}
