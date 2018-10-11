package shadowsocks

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"

	sscore "github.com/shadowsocks/go-shadowsocks2/core"
	sssocks "github.com/shadowsocks/go-shadowsocks2/socks"

	tun2socks "github.com/eycorsican/go-tun2socks"
	"github.com/eycorsican/go-tun2socks/lwip"
)

type udpHandler struct {
	sync.Mutex

	cipher      sscore.Cipher
	remoteAddr  net.Addr
	conns       map[tun2socks.Connection]net.PacketConn
	targetAddrs map[tun2socks.Connection]sssocks.Addr
}

func NewUDPHandler(server, cipher, password string) tun2socks.ConnectionHandler {
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
		conns:       make(map[tun2socks.Connection]net.PacketConn, 16),
		targetAddrs: make(map[tun2socks.Connection]sssocks.Addr, 16),
	}
}

func (h *udpHandler) fetchUDPInput(conn tun2socks.Connection, input net.PacketConn) {
	buf := lwip.NewBytes(lwip.BufSize)

	defer func() {
		h.Close(conn)
		lwip.FreeBytes(buf)
	}()

	for {
		input.SetDeadline(time.Time{})
		n, _, err := input.ReadFrom(buf)
		if err != nil {
			log.Printf("failed to read UDP data from Shadowsocks server: %v", err)
			return
		}

		addr := sssocks.SplitAddr(buf[:])
		err = conn.Write(buf[int(len(addr)):n])
		if err != nil {
			log.Printf("failed to write UDP data to TUN")
			return
		}
	}
}

func (h *udpHandler) Connect(conn tun2socks.Connection, target net.Addr) error {
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
	return nil
}

func (h *udpHandler) DidReceive(conn tun2socks.Connection, data []byte) error {
	pc, ok1 := h.conns[conn]
	targetAddr, ok2 := h.targetAddrs[conn]
	if ok1 && ok2 {
		buf := append([]byte{0, 0, 0}, targetAddr...)
		buf = append(buf, data[:]...)
		_, err := pc.WriteTo(buf[3:], h.remoteAddr)
		if err != nil {
			log.Printf("failed to write UDP payload to Shadowsocks server: %v", err)
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

	if pc, ok := h.conns[conn]; ok {
		pc.Close()
		delete(h.conns, conn)
	}
	delete(h.targetAddrs, conn)
}
