package direct

import (
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	tun2socks "github.com/eycorsican/go-tun2socks"
	"github.com/eycorsican/go-tun2socks/lwip"
)

type udpHandler struct {
	sync.Mutex

	udpConns       map[tun2socks.Connection]*net.UDPConn
	udpTargetAddrs map[tun2socks.Connection]*net.UDPAddr
}

func NewUDPHandler() tun2socks.ConnectionHandler {
	return &udpHandler{
		udpConns:       make(map[tun2socks.Connection]*net.UDPConn, 8),
		udpTargetAddrs: make(map[tun2socks.Connection]*net.UDPAddr, 8),
	}
}

func (h *udpHandler) fetchUDPInput(conn tun2socks.Connection, pc *net.UDPConn) {
	buf := lwip.NewBytes(lwip.BufSize)

	defer func() {
		h.Close(conn)
		lwip.FreeBytes(buf)
	}()

	for {
		pc.SetDeadline(time.Now().Add(16 * time.Second))
		n, _, err := pc.ReadFromUDP(buf)
		if err != nil {
			log.Printf("failed to read UDP data from remote: %v", err)
			return
		}

		_, err = conn.Write(buf[:n])
		if err != nil {
			log.Printf("failed to write UDP data to TUN")
			return
		}
	}
}

func (h *udpHandler) Connect(conn tun2socks.Connection, target net.Addr) error {
	bindAddr := &net.UDPAddr{IP: net.IP{0, 0, 0, 0}, Port: 0}
	pc, err := net.ListenUDP("udp", bindAddr)
	if err != nil {
		log.Printf("failed to bind udp address")
		return err
	}
	tgtAddr, _ := net.ResolveUDPAddr(target.Network(), target.String())
	h.Lock()
	h.udpTargetAddrs[conn] = tgtAddr
	h.udpConns[conn] = pc
	h.Unlock()
	go h.fetchUDPInput(conn, pc)
	return nil
}

func (h *udpHandler) DidReceive(conn tun2socks.Connection, data []byte) error {
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

func (h *udpHandler) DidSend(conn tun2socks.Connection, len uint16) {
	// unused
}

func (h *udpHandler) DidClose(conn tun2socks.Connection) {
	// unused
}

func (h *udpHandler) LocalDidClose(conn tun2socks.Connection) {
	// unused
}

func (h *udpHandler) Close(conn tun2socks.Connection) {
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
