package redirect

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/eycorsican/go-tun2socks/core"
)

type udpHandler struct {
	sync.Mutex

	timeout        time.Duration
	udpConns       map[core.Connection]*net.UDPConn
	udpTargetAddrs map[core.Connection]*net.UDPAddr
	target         string
}

func NewUDPHandler(target string, timeout time.Duration) core.ConnectionHandler {
	return &udpHandler{
		timeout:        timeout,
		udpConns:       make(map[core.Connection]*net.UDPConn, 8),
		udpTargetAddrs: make(map[core.Connection]*net.UDPAddr, 8),
		target:         target,
	}
}

func (h *udpHandler) fetchUDPInput(conn core.Connection, pc *net.UDPConn) {
	buf := core.NewBytes(core.BufSize)

	defer func() {
		h.Close(conn)
		core.FreeBytes(buf)
	}()

	for {
		pc.SetDeadline(time.Now().Add(h.timeout))
		n, _, err := pc.ReadFromUDP(buf)
		if err != nil {
			// log.Printf("failed to read UDP data from remote: %v", err)
			return
		}

		_, err = conn.Write(buf[:n])
		if err != nil {
			log.Printf("failed to write UDP data to TUN")
			return
		}
	}
}

func (h *udpHandler) Connect(conn core.Connection, target net.Addr) error {
	bindAddr := &net.UDPAddr{IP: nil, Port: 0}
	pc, err := net.ListenUDP("udp", bindAddr)
	if err != nil {
		log.Printf("failed to bind udp address")
		return err
	}
	tgtAddr, _ := net.ResolveUDPAddr("udp", h.target)
	h.Lock()
	h.udpTargetAddrs[conn] = tgtAddr
	h.udpConns[conn] = pc
	h.Unlock()
	go h.fetchUDPInput(conn, pc)
	log.Printf("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}

func (h *udpHandler) DidReceive(conn core.Connection, data []byte) error {
	h.Lock()
	pc, ok1 := h.udpConns[conn]
	addr, ok2 := h.udpTargetAddrs[conn]
	h.Unlock()

	if ok1 && ok2 {
		_, err := pc.WriteToUDP(data, addr)
		if err != nil {
			log.Printf("failed to write UDP payload to SOCKS5 server: %v", err)
			return errors.New("failed to write UDP data")
		}
		return nil
	} else {
		return errors.New(fmt.Sprintf("proxy connection %v->%v does not exists", conn.LocalAddr(), conn.RemoteAddr()))
	}
}

func (h *udpHandler) DidSend(conn core.Connection, len uint16) {
	// unused
}

func (h *udpHandler) DidClose(conn core.Connection) {
	// unused
}

func (h *udpHandler) LocalDidClose(conn core.Connection) {
	// unused
}

func (h *udpHandler) Close(conn core.Connection) {
	conn.Close()

	h.Lock()
	defer h.Unlock()

	if _, ok := h.udpTargetAddrs[conn]; ok {
		delete(h.udpTargetAddrs, conn)
	}
	if pc, ok := h.udpConns[conn]; ok {
		pc.Close()
		delete(h.udpConns, conn)
	}
}
