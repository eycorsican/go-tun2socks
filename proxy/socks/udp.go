package socks

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy"
)

type udpHandler struct {
	sync.Mutex

	proxyHost   string
	proxyPort   uint16
	udpConns    map[core.Connection]net.Conn
	tcpConns    map[core.Connection]net.Conn
	targetAddrs map[core.Connection]Addr
	dnsCache    *proxy.DNSCache
	timeout     time.Duration
}

func NewUDPHandler(proxyHost string, proxyPort uint16, timeout time.Duration, dnsCache *proxy.DNSCache) core.ConnectionHandler {
	return &udpHandler{
		proxyHost:   proxyHost,
		proxyPort:   proxyPort,
		udpConns:    make(map[core.Connection]net.Conn, 8),
		tcpConns:    make(map[core.Connection]net.Conn, 8),
		targetAddrs: make(map[core.Connection]Addr, 8),
		dnsCache:    dnsCache,
		timeout:     timeout,
	}
}

func (h *udpHandler) handleTCP(conn core.Connection, c net.Conn) {
	buf := core.NewBytes(core.BufSize)
	defer core.FreeBytes(buf)

	for {
		c.SetDeadline(time.Time{})
		_, err := c.Read(buf)
		if err == io.EOF {
			log.Printf("UDP associate to %v closed by remote", c.RemoteAddr())
			h.Close(conn)
			return
		} else if err != nil {
			h.Close(conn)
			return
		}
	}
}

func (h *udpHandler) fetchUDPInput(conn core.Connection, input net.Conn) {
	buf := core.NewBytes(core.BufSize)

	defer func() {
		h.Close(conn)
		core.FreeBytes(buf)
	}()

	for {
		input.SetDeadline(time.Now().Add(h.timeout))
		n, err := input.Read(buf)
		if err != nil {
			// log.Printf("read remote failed: %v", err)
			return
		}

		addr := SplitAddr(buf[3:])
		_, err = conn.Write(buf[int(3+len(addr)):n])
		if err != nil {
			log.Printf("write local failed: %v", err)
			return
		}

		if h.dnsCache != nil {
			h.Lock()
			targetAddr, ok2 := h.targetAddrs[conn]
			h.Unlock()
			if ok2 {
				_, port, err := net.SplitHostPort(targetAddr.String())
				if err != nil {
					log.Fatal("impossible error")
				}
				if port == strconv.Itoa(proxy.COMMON_DNS_PORT) {
					h.dnsCache.Store(buf[int(3+len(addr)):n])
					return // DNS response
				}
			}
		}
	}
}

func (h *udpHandler) Connect(conn core.Connection, target net.Addr) error {
	c, err := net.Dial("tcp", core.ParseTCPAddr(h.proxyHost, h.proxyPort).String())
	if err != nil {
		return err
	}

	// send VER, NMETHODS, METHODS
	c.Write([]byte{5, 1, 0})

	buf := make([]byte, MaxAddrLen)
	// read VER METHOD
	if _, err := io.ReadFull(c, buf[:2]); err != nil {
		return err
	}

	targetAddr := ParseAddr(target.String())
	// write VER CMD RSV ATYP DST.ADDR DST.PORT
	c.Write(append([]byte{5, socks5UDPAssociate, 0}, targetAddr...))

	// read VER REP RSV ATYP BND.ADDR BND.PORT
	if _, err := io.ReadFull(c, buf[:3]); err != nil {
		return err
	}

	rep := buf[1]
	if rep != 0 {
		return errors.New("SOCKS handshake failed")
	}

	remoteAddr, err := readAddr(c, buf)
	if err != nil {
		return err
	}

	go h.handleTCP(conn, c)

	pc, err := net.Dial("udp", remoteAddr.String())
	if err != nil {
		return err
	}

	h.Lock()
	h.tcpConns[conn] = c
	h.udpConns[conn] = pc
	h.targetAddrs[conn] = targetAddr
	h.Unlock()
	go h.fetchUDPInput(conn, pc)
	log.Printf("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}

func (h *udpHandler) DidReceive(conn core.Connection, data []byte) error {
	h.Lock()
	pc, ok1 := h.udpConns[conn]
	targetAddr, ok2 := h.targetAddrs[conn]
	h.Unlock()

	if ok2 && h.dnsCache != nil {
		_, port, err := net.SplitHostPort(targetAddr.String())
		if err != nil {
			log.Fatal("impossible error")
		}
		if port == strconv.Itoa(proxy.COMMON_DNS_PORT) {
			if answer := h.dnsCache.Query(data); answer != nil {
				var buf [1024]byte
				if dnsAnswer, err := answer.PackBuffer(buf[:]); err == nil {
					_, err = conn.Write(dnsAnswer)
					if err != nil {
						return errors.New(fmt.Sprintf("cache dns answer failed: %v", err))
					}
					h.Close(conn)
					return nil
				}
			}
		}
	}

	if ok1 && ok2 {
		buf := append([]byte{0, 0, 0}, targetAddr...)
		buf = append(buf, data[:]...)
		_, err := pc.Write(buf)
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

	if c, ok := h.tcpConns[conn]; ok {
		c.Close()
		delete(h.tcpConns, conn)
	}
	if pc, ok := h.udpConns[conn]; ok {
		pc.Close()
		delete(h.udpConns, conn)
	}
	delete(h.targetAddrs, conn)
}
