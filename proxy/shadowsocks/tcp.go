package shadowsocks

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	sscore "github.com/shadowsocks/go-shadowsocks2/core"
	sssocks "github.com/shadowsocks/go-shadowsocks2/socks"

	"github.com/eycorsican/go-tun2socks/common/dns"
	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct {
	sync.Mutex

	cipher sscore.Cipher
	server string
	conns  map[core.TCPConn]net.Conn

	fakeDns dns.FakeDns
}

func (h *tcpHandler) fetchInput(conn core.TCPConn, input io.Reader) {
	defer func() {
		h.Close(conn)
		conn.Close() // also close tun2socks connection here
	}()

	_, err := io.Copy(conn, input)
	if err != nil {
		// log.Printf("fetch input failed: %v", err)
		return
	}
}

func NewTCPHandler(server, cipher, password string, fakeDns dns.FakeDns) core.TCPConnHandler {
	ciph, err := sscore.PickCipher(cipher, []byte{}, password)
	if err != nil {
		log.Errorf("failed to pick a cipher: %v", err)
	}

	return &tcpHandler{
		cipher:  ciph,
		server:  server,
		conns:   make(map[core.TCPConn]net.Conn, 16),
		fakeDns: fakeDns,
	}
}

func (h *tcpHandler) Connect(conn core.TCPConn, target net.Addr) error {
	if target == nil {
		log.Fatalf("unexpected nil target")
	}

	// Connect the relay server.
	rc, err := net.Dial("tcp", h.server)
	if err != nil {
		return errors.New(fmt.Sprintf("dial remote server failed: %v", err))
	}
	rc = h.cipher.StreamConn(rc)

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

	// Write target address.
	tgt := sssocks.ParseAddr(dest)
	_, err = rc.Write(tgt)
	if err != nil {
		return fmt.Errorf("send target address failed: %v", err)
	}

	h.Lock()
	h.conns[conn] = rc
	h.Unlock()

	go h.fetchInput(conn, rc)

	log.Infof("new proxy connection for target: %s:%s", target.Network(), dest)
	return nil
}

func (h *tcpHandler) DidReceive(conn core.TCPConn, data []byte) error {
	h.Lock()
	rc, ok1 := h.conns[conn]
	h.Unlock()

	if ok1 {
		_, err := rc.Write(data)
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

func (h *tcpHandler) DidClose(conn core.TCPConn) {
	h.Close(conn)
}

func (h *tcpHandler) LocalDidClose(conn core.TCPConn) {
	h.Close(conn)
}

func (h *tcpHandler) Close(conn core.TCPConn) {
	h.Lock()
	defer h.Unlock()

	if rc, found := h.conns[conn]; found {
		rc.Close()
	}
	delete(h.conns, conn)
}
