package shadowsocks

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	sscore "github.com/shadowsocks/go-shadowsocks2/core"
	sssocks "github.com/shadowsocks/go-shadowsocks2/socks"

	"github.com/eycorsican/go-tun2socks/common/dns"
	"github.com/eycorsican/go-tun2socks/core"
)

type udpHandler struct {
	sync.Mutex

	cipher     sscore.Cipher
	remoteAddr net.Addr
	conns      map[core.UDPConn]net.PacketConn
	dnsCache   dns.DnsCache
	timeout    time.Duration
}

func NewUDPHandler(server, cipher, password string, timeout time.Duration, dnsCache dns.DnsCache) core.UDPConnHandler {
	ciph, err := sscore.PickCipher(cipher, []byte{}, password)
	if err != nil {
		log.Fatal(err)
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		log.Fatal(err)
	}

	return &udpHandler{
		cipher:     ciph,
		remoteAddr: remoteAddr,
		conns:      make(map[core.UDPConn]net.PacketConn, 16),
		dnsCache:   dnsCache,
		timeout:    timeout,
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

		addr := sssocks.SplitAddr(buf[:])
		resolvedAddr, err := net.ResolveUDPAddr("udp", addr.String())
		if err != nil {
			return
		}
		_, err = conn.WriteFrom(buf[int(len(addr)):n], resolvedAddr)
		if err != nil {
			log.Printf("write local failed: %v", err)
			return
		}

		if h.dnsCache != nil {
			_, port, err := net.SplitHostPort(addr.String())
			if err != nil {
				log.Fatal("impossible error")
			}
			if port == strconv.Itoa(dns.COMMON_DNS_PORT) {
				h.dnsCache.Store(buf[int(len(addr)):n])
				return // DNS response
			}
		}
	}
}

func (h *udpHandler) Connect(conn core.UDPConn, target net.Addr) error {
	pc, err := net.ListenPacket("udp", "")
	if err != nil {
		return err
	}
	pc = h.cipher.PacketConn(pc)

	h.Lock()
	h.conns[conn] = pc
	h.Unlock()
	go h.fetchUDPInput(conn, pc)
	if target != nil {
		log.Printf("new proxy connection for target: %s:%s", target.Network(), target.String())
	}
	return nil
}

func (h *udpHandler) DidReceiveTo(conn core.UDPConn, data []byte, addr net.Addr) error {
	h.Lock()
	pc, ok1 := h.conns[conn]
	h.Unlock()

	if h.dnsCache != nil {
		_, port, err := net.SplitHostPort(addr.String())
		if err != nil {
			log.Fatal("impossible error")
		}
		if port == strconv.Itoa(dns.COMMON_DNS_PORT) {
			if answer := h.dnsCache.Query(data); answer != nil {
				_, err = conn.WriteFrom(answer, addr)
				if err != nil {
					return errors.New(fmt.Sprintf("cache dns answer failed: %v", err))
				}
				h.Close(conn)
				return nil
			}
		}
	}

	if ok1 {
		buf := append([]byte{0, 0, 0}, sssocks.ParseAddr(addr.String())...)
		buf = append(buf, data[:]...)
		_, err := pc.WriteTo(buf[3:], h.remoteAddr)
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

	if pc, ok := h.conns[conn]; ok {
		pc.Close()
		delete(h.conns, conn)
	}
}
