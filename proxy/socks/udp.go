package socks

import (
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/eycorsican/go-tun2socks/common/dns"
	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

type udpHandler struct {
	sync.Mutex

	proxyHost   string
	proxyPort   uint16
	udpConns    map[core.UDPConn]net.PacketConn
	tcpConns    map[core.UDPConn]net.Conn
	remoteAddrs map[core.UDPConn]*net.UDPAddr // UDP relay server addresses
	dnsCache    dns.DnsCache
	fakeDns     dns.FakeDns
	timeout     time.Duration
}

func NewUDPHandler(proxyHost string, proxyPort uint16, timeout time.Duration, dnsCache dns.DnsCache, fakeDns dns.FakeDns) core.UDPConnHandler {
	return &udpHandler{
		proxyHost:   proxyHost,
		proxyPort:   proxyPort,
		udpConns:    make(map[core.UDPConn]net.PacketConn, 8),
		tcpConns:    make(map[core.UDPConn]net.Conn, 8),
		remoteAddrs: make(map[core.UDPConn]*net.UDPAddr, 8),
		dnsCache:    dnsCache,
		fakeDns:     fakeDns,
		timeout:     timeout,
	}
}

func (h *udpHandler) handleTCP(conn core.UDPConn, c net.Conn) {
	buf := core.NewBytes(core.BufSize)
	defer core.FreeBytes(buf)

	for {
		c.SetDeadline(time.Time{})
		_, err := c.Read(buf)
		if err == io.EOF {
			log.Warnf("UDP associate to %v closed by remote", c.RemoteAddr())
			h.Close(conn)
			return
		} else if err != nil {
			h.Close(conn)
			return
		}
	}
}

func (h *udpHandler) fetchUDPInput(conn core.UDPConn, input net.PacketConn) {
	buf := core.NewBytes(core.BufSize)

	defer func() {
		h.Close(conn)
		core.FreeBytes(buf)
	}()

	for {
		input.SetDeadline(time.Now().Add(h.timeout))
		n, _, err := input.ReadFrom(buf)
		if err != nil {
			// log.Printf("read remote failed: %v", err)
			return
		}

		addr := SplitAddr(buf[3:])
		resolvedAddr, err := net.ResolveUDPAddr("udp", addr.String())
		if err != nil {
			return
		}
		_, err = conn.WriteFrom(buf[int(3+len(addr)):n], resolvedAddr)
		if err != nil {
			log.Warnf("write local failed: %v", err)
			return
		}

		if h.dnsCache != nil {
			_, port, err := net.SplitHostPort(addr.String())
			if err != nil {
				panic("impossible error")
			}
			if port == strconv.Itoa(dns.COMMON_DNS_PORT) {
				h.dnsCache.Store(buf[int(3+len(addr)):n])
				return // DNS response
			}
		}
	}
}

func (h *udpHandler) Connect(conn core.UDPConn, target net.Addr) error {
	if target == nil {
		return h.connectInternal(conn, "")
	}

	// Replace with a domain name if target address IP is a fake IP.
	host, port, err := net.SplitHostPort(target.String())
	if err != nil {
		log.Errorf("error when split host port %v", err)
	}
	var targetHost string = host
	if h.fakeDns != nil {
		_, port, err := net.SplitHostPort(target.String())
		if err != nil {
			panic("impossible error")
		}
		if port == strconv.Itoa(dns.COMMON_DNS_PORT) {
			return nil // skip dns
		}
		if ip := net.ParseIP(host); ip != nil {
			if h.fakeDns.IsFakeIP(ip) {
				targetHost = h.fakeDns.QueryDomain(ip)
			}
		}
	}
	dest := fmt.Sprintf("%s:%s", targetHost, port)

	return h.connectInternal(conn, dest)
}

func (h *udpHandler) connectInternal(conn core.UDPConn, dest string) error {
	c, err := net.DialTimeout("tcp", core.ParseTCPAddr(h.proxyHost, h.proxyPort).String(), 4*time.Second)
	if err != nil {
		return err
	}
	c.SetDeadline(time.Now().Add(4 * time.Second))

	// send VER, NMETHODS, METHODS
	c.Write([]byte{5, 1, 0})

	buf := make([]byte, MaxAddrLen)
	// read VER METHOD
	if _, err := io.ReadFull(c, buf[:2]); err != nil {
		return err
	}

	if len(dest) != 0 {
		targetAddr := ParseAddr(dest)
		// write VER CMD RSV ATYP DST.ADDR DST.PORT
		c.Write(append([]byte{5, socks5UDPAssociate, 0}, targetAddr...))
	} else {
		c.Write(append([]byte{5, socks5UDPAssociate, 0}, []byte{1, 0, 0, 0, 0, 0, 0}...))
	}

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

	resolvedRemoteAddr, err := net.ResolveUDPAddr("udp", remoteAddr.String())
	if err != nil {
		return errors.New("failed to resolve remote address")
	}

	go h.handleTCP(conn, c)

	pc, err := net.ListenPacket("udp", "")
	if err != nil {
		return err
	}

	h.Lock()
	h.tcpConns[conn] = c
	h.udpConns[conn] = pc
	h.remoteAddrs[conn] = resolvedRemoteAddr
	h.Unlock()
	go h.fetchUDPInput(conn, pc)
	if len(dest) != 0 {
		log.Infof("new proxy connection for target: udp:%s", dest)
	}
	return nil
}

func (h *udpHandler) DidReceiveTo(conn core.UDPConn, data []byte, addr net.Addr) error {
	h.Lock()
	pc, ok1 := h.udpConns[conn]
	remoteAddr, ok2 := h.remoteAddrs[conn]
	h.Unlock()

	if h.fakeDns != nil {
		_, port, err := net.SplitHostPort(addr.String())
		if err != nil {
			panic("impossible error")
		}
		if port == strconv.Itoa(dns.COMMON_DNS_PORT) {
			resp, err := h.fakeDns.GenerateFakeResponse(data)
			if err != nil {
				// FIXME This will block the lwip thread, need to optimize.
				if err := h.connectInternal(conn, addr.String()); err != nil {
					return fmt.Errorf("failed to connect to %v:%v", addr.Network(), addr.String())
				}
				h.Lock()
				pc, ok1 = h.udpConns[conn]
				remoteAddr, ok2 = h.remoteAddrs[conn]
				h.Unlock()
			} else {
				_, err = conn.WriteFrom(resp, addr)
				if err != nil {
					return errors.New(fmt.Sprintf("write dns answer failed: %v", err))
				}
				h.Close(conn)
				return nil
			}
		}
	}

	if h.dnsCache != nil {
		_, port, err := net.SplitHostPort(addr.String())
		if err != nil {
			panic("impossible error")
		}
		if port == strconv.Itoa(dns.COMMON_DNS_PORT) {
			if answer := h.dnsCache.Query(data); answer != nil {
				_, err = conn.WriteFrom(answer, addr)
				if err != nil {
					return errors.New(fmt.Sprintf("write dns answer failed: %v", err))
				}
				h.Close(conn)
				return nil
			}
		}
	}

	if ok1 && ok2 {
		// Replace with a domain name if target address IP is a fake IP.
		host, port, err := net.SplitHostPort(addr.String())
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

		buf := append([]byte{0, 0, 0}, ParseAddr(dest)...)
		buf = append(buf, data[:]...)
		_, err = pc.WriteTo(buf, remoteAddr)
		if err != nil {
			h.Close(conn)
			return errors.New(fmt.Sprintf("write remote failed: %v", err))
		}
		return nil
	} else {
		h.Close(conn)
		return errors.New(fmt.Sprintf("proxy connection %v->%v does not exists", conn.LocalAddr(), addr))
	}
}

func (h *udpHandler) Close(conn core.UDPConn) {
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
	delete(h.remoteAddrs, conn)
}
