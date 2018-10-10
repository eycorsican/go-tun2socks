package socks

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	tun2socks "github.com/eycorsican/go-tun2socks"
	"github.com/eycorsican/go-tun2socks/lwip"
)

type udpHandler struct {
	sync.Mutex

	proxyHost  string
	proxyPort  uint16
	udpConns   map[tun2socks.Connection]net.Conn
	tcpConns   map[tun2socks.Connection]net.Conn
	targetAddr Addr
}

func NewUDPHandler(proxyHost string, proxyPort uint16) tun2socks.ConnectionHandler {
	return &udpHandler{
		proxyHost: proxyHost,
		proxyPort: proxyPort,
		udpConns:  make(map[tun2socks.Connection]net.Conn, 8),
		tcpConns:  make(map[tun2socks.Connection]net.Conn, 8),
	}
}

func (h *udpHandler) handleTCP(conn tun2socks.Connection, c net.Conn) {
	var buf = make([]byte, 1)
	for {
		_, err := c.Read(buf)
		if err != nil {
			h.Close(conn)
			return
		}
	}
}

func (h *udpHandler) fetchUDPInput(conn tun2socks.Connection, input net.Conn) {
	buf := lwip.NewBytes(lwip.BufSize)

	defer func() {
		h.Close(conn)
		lwip.FreeBytes(buf)
	}()

	for {
		input.SetDeadline(time.Time{})
		n, err := input.Read(buf)
		if err != nil {
			log.Printf("failed to read UDP data from SOCKS5 server: %v", err)
			return
		}

		// no copy
		addr := SplitAddr(buf[3:])
		err = conn.Write(buf[int(3+len(addr)):n])
		if err != nil {
			log.Printf("failed to write UDP data to TUN")
			return
		}
	}
}

func (h *udpHandler) Connect(conn tun2socks.Connection, target net.Addr) {
	c, err := net.Dial("tcp", fmt.Sprintf("%s:%d", h.proxyHost, h.proxyPort))
	if err != nil {
		log.Printf("failed to dial TCP while creating SOCKS5 UDP associate connection")
		return
	}

	// send VER, NMETHODS, METHODS
	c.Write([]byte{5, 1, 0})

	buf := make([]byte, MaxAddrLen)
	// read VER METHOD
	if _, err := io.ReadFull(c, buf[:2]); err != nil {
		log.Printf("failed to read SOCKS5 version message reply", err)
		return
	}

	h.targetAddr = ParseAddr(target.String())
	// write VER CMD RSV ATYP DST.ADDR DST.PORT
	c.Write(append([]byte{5, socks5UDPAssociate, 0}, h.targetAddr...))

	// read VER REP RSV ATYP BND.ADDR BND.PORT
	if _, err := io.ReadFull(c, buf[:3]); err != nil {
		log.Printf("failed to read SOCKS5 request reply", err)
		return
	}

	rep := buf[1]
	if rep != 0 {
		log.Printf("SOCKS5 server reply: %d, not succeeded", rep)
		return
	}

	uAddr, err := readAddr(c, buf)
	if err != nil {
		log.Printf("failed to read UDP assiciate server address")
		return
	}

	go h.handleTCP(conn, c)

	pc, err := net.Dial("udp", uAddr.String())
	if err != nil {
		log.Printf("failed to dial UDP associate server")
		return
	}
	log.Printf("dialed UDP connection: %v -> %v", pc.LocalAddr(), pc.RemoteAddr())

	h.Lock()
	h.tcpConns[conn] = c
	h.udpConns[conn] = pc
	h.Unlock()
	go h.fetchUDPInput(conn, pc)
}

func (h *udpHandler) DidReceive(conn tun2socks.Connection, data []byte) error {
	if pc, ok := h.udpConns[conn]; ok {
		buf := append([]byte{0, 0, 0}, h.targetAddr...)
		buf = append(buf, data[:]...)
		_, err := pc.Write(buf)
		if err != nil {
			log.Printf("failed to write UDP payload to SOCKS5 server: %v", err)
			h.Close(conn)
			return errors.New("failed to write data")
		}
		return nil
	} else {
		h.Close(conn)
		return errors.New("connection does not exists")
	}
}

func (h *udpHandler) DidSend(conn tun2socks.Connection, len uint16) {
	// unused
}

func (h *udpHandler) DidClose(conn tun2socks.Connection) {
	// unused
}

func (h *udpHandler) DidAbort(conn tun2socks.Connection) {
	// unused
}

func (h *udpHandler) DidReset(conn tun2socks.Connection) {
	// unused
}

func (h *udpHandler) LocalDidClose(conn tun2socks.Connection) {
	// unused
}

func (h *udpHandler) Close(conn tun2socks.Connection) {
	conn.Close()

	h.Lock()
	defer h.Unlock()

	if c, ok := h.tcpConns[conn]; ok {
		c.Close()
		delete(h.tcpConns, conn)
	}
	if pc, ok := h.udpConns[conn]; ok {
		pc.Close()
		delete(h.udpConns, conn)
	}
}
