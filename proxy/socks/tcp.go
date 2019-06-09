package socks

import (
	"io"
	"net"
	"sync"

	"golang.org/x/net/proxy"

	"github.com/eycorsican/go-tun2socks/common/dns"
	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct {
	sync.Mutex

	proxyHost string
	proxyPort uint16

	fakeDns dns.FakeDns
}

func NewTCPHandler(proxyHost string, proxyPort uint16, fakeDns dns.FakeDns) core.TCPConnHandler {
	return &tcpHandler{
		proxyHost: proxyHost,
		proxyPort: proxyPort,
		fakeDns:   fakeDns,
	}
}

func (h *tcpHandler) handleInput(conn net.Conn, input io.ReadCloser) {
	defer func() {
		conn.Close()
		input.Close()
	}()
	io.Copy(conn, input)
}

func (h *tcpHandler) handleOutput(conn net.Conn, output io.WriteCloser) {
	defer func() {
		conn.Close()
		output.Close()
	}()
	io.Copy(output, conn)
}

func (h *tcpHandler) Handle(conn net.Conn, target net.Addr) error {
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

	c, err := dialer.Dial(target.Network(), target.String())
	if err != nil {
		return err
	}
	go h.handleInput(conn, c)
	go h.handleOutput(conn, c)

	log.Access("proxy", target.Network(), conn.LocalAddr().String(), target.String())

	return nil
}
