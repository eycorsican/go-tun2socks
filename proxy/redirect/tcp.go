package redirect

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

// To do a benchmark using iperf3 locally, you may follow these steps:
//
// 1. Setup and configure the TUN device and start tun2socks with the
//    redirect handler using the following command:
//      tun2socks -proxyType redirect -proxyServer 127.0.0.1:1234
//    Tun2socks will redirect all traffic to 127.0.0.1:1234.
//
// 2. Route traffic targeting 1.2.3.4 to the TUN interface (240.0.0.1):
//      route add 1.2.3.4/32 240.0.0.1
//
// 3. Run iperf3 server locally and listening on 1234 port:
//      iperf3 -s -p 1234
//
// 4. Run iperf3 client locally and connect to 1.2.3.4:1234:
//      iperf3 -c 1.2.3.4 -p 1234
//
// It works this way:
// iperf3 client -> 1.2.3.4:1234 -> routing table -> TUN (240.0.0.1) -> tun2socks -> tun2socks redirect anything to 127.0.0.1:1234 -> iperf3 server
//
type tcpHandler struct {
	sync.Mutex
	conns  map[core.TCPConn]net.Conn
	target string
}

func NewTCPHandler(target string) core.TCPConnHandler {
	return &tcpHandler{
		conns:  make(map[core.TCPConn]net.Conn, 16),
		target: target,
	}
}

func (h *tcpHandler) fetchInput(conn core.TCPConn, input io.Reader) {
	_, err := io.Copy(conn, input)
	if err != nil {
		h.Close(conn)
		conn.Close()
	}
}

func (h *tcpHandler) Connect(conn core.TCPConn, target net.Addr) error {
	c, err := net.Dial("tcp", h.target)
	if err != nil {
		return err
	}
	h.Lock()
	h.conns[conn] = c
	h.Unlock()
	c.SetReadDeadline(time.Time{})
	go h.fetchInput(conn, c)
	log.Infof("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}

func (h *tcpHandler) DidReceive(conn core.TCPConn, data []byte) error {
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

func (h *tcpHandler) DidClose(conn core.TCPConn) {
	h.Close(conn)
}

func (h *tcpHandler) LocalDidClose(conn core.TCPConn) {
	h.Close(conn)
}

func (h *tcpHandler) Close(conn core.TCPConn) {
	h.Lock()
	defer h.Unlock()

	if c, found := h.conns[conn]; found {
		c.Close()
	}
	delete(h.conns, conn)
}
