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

	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy"
)

type udpHandler struct {
	sync.Mutex

	cipher      sscore.Cipher
	remoteAddr  net.Addr
	conns       map[core.Connection]net.PacketConn
	targetAddrs map[core.Connection]sssocks.Addr
	dnsCache    *proxy.DNSCache
	timeout     time.Duration
}

func NewUDPHandler(server, cipher, password string, timeout time.Duration, dnsCache *proxy.DNSCache) core.ConnectionHandler {
	ciph, err := sscore.PickCipher(cipher, []byte{}, password)
	if err != nil {
		log.Fatal(err)
	}

	remoteAddr, err := net.ResolveUDPAddr("udp", server)
	if err != nil {
		log.Fatal(err)
	}

	return &udpHandler{
		cipher:      ciph,
		remoteAddr:  remoteAddr,
		conns:       make(map[core.Connection]net.PacketConn, 16),
		targetAddrs: make(map[core.Connection]sssocks.Addr, 16),
		dnsCache:    dnsCache,
		timeout:     timeout,
	}
}

func (h *udpHandler) fetchUDPInput(conn core.Connection, input net.PacketConn) {
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
		_, err = conn.Write(buf[int(len(addr)):n])
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
					h.dnsCache.Store(buf[int(len(addr)):n])
					return // DNS response
				}
			}
		}
	}
}

func (h *udpHandler) Connect(conn core.Connection, target net.Addr) error {
	pc, err := net.ListenPacket("udp", "")
	if err != nil {
		return err
	}
	pc = h.cipher.PacketConn(pc)

	h.Lock()
	h.conns[conn] = pc
	h.targetAddrs[conn] = sssocks.ParseAddr(target.String())
	h.Unlock()
	go h.fetchUDPInput(conn, pc)
	log.Printf("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}

func (h *udpHandler) DidReceive(conn core.Connection, data []byte) error {
	h.Lock()
	pc, ok1 := h.conns[conn]
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
		_, err := pc.WriteTo(buf[3:], h.remoteAddr)
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

	if pc, ok := h.conns[conn]; ok {
		pc.Close()
		delete(h.conns, conn)
	}
	delete(h.targetAddrs, conn)
}
