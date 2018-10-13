package shadowsocks

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	sscore "github.com/shadowsocks/go-shadowsocks2/core"
	sssocks "github.com/shadowsocks/go-shadowsocks2/socks"

	tun2socks "github.com/eycorsican/go-tun2socks"
)

type tcpHandler struct {
	sync.Mutex

	cipher   sscore.Cipher
	server   string
	conns    map[tun2socks.Connection]net.Conn
	tgtAddrs map[tun2socks.Connection]net.Addr
	tgtSent  map[tun2socks.Connection]bool
}

func (h *tcpHandler) fetchInput(conn tun2socks.Connection, input io.Reader) {
	defer func() {
		h.Close(conn)
		conn.Close() // also close tun2socks connection here
	}()

	_, err := io.Copy(conn.(io.Writer), input)
	if err != nil {
		log.Printf("fetch input failed: %v", err)
	}
}

func (h *tcpHandler) sendTargetAddress(conn tun2socks.Connection) error {
	h.Lock()
	defer h.Unlock()

	tgtAddr, ok1 := h.tgtAddrs[conn]
	rc, ok2 := h.conns[conn]
	sent, ok3 := h.tgtSent[conn]
	if ok3 && sent {
		return nil
	}
	if ok1 && ok2 {
		tgt := sssocks.ParseAddr(tgtAddr.String())
		_, err := rc.Write(tgt)
		if err != nil {
			return errors.New(fmt.Sprintf("send target address failed: %v", err))
		}
		h.tgtSent[conn] = true
		go h.fetchInput(conn, rc)
	} else {
		return errors.New("target address not found")
	}
	return nil
}

func NewTCPHandler(server, cipher, password string) tun2socks.ConnectionHandler {
	ciph, err := sscore.PickCipher(cipher, []byte{}, password)
	if err != nil {
		log.Fatal(err)
	}

	return &tcpHandler{
		cipher:   ciph,
		server:   server,
		conns:    make(map[tun2socks.Connection]net.Conn, 16),
		tgtAddrs: make(map[tun2socks.Connection]net.Addr, 16),
		tgtSent:  make(map[tun2socks.Connection]bool, 16),
	}
}

func (h *tcpHandler) Connect(conn tun2socks.Connection, target net.Addr) error {
	rc, err := net.Dial("tcp", h.server)
	if err != nil {
		return errors.New(fmt.Sprintf("dial remote server failed: %v", err))
	}
	rc = h.cipher.StreamConn(rc)

	h.Lock()
	h.conns[conn] = rc
	h.tgtAddrs[conn] = target
	h.Unlock()
	rc.SetDeadline(time.Time{})
	return nil
}

func (h *tcpHandler) DidReceive(conn tun2socks.Connection, data []byte) error {
	h.Lock()
	rc, ok1 := h.conns[conn]
	h.Unlock()

	if ok1 {
		err := h.sendTargetAddress(conn)
		if err != nil {
			h.Close(conn)
			return err
		}
		_, err = rc.Write(data)
		if err != nil {
			h.Close(conn)
			return errors.New(fmt.Sprintf("write remote failed: %v", err))
		}
		return nil
	} else {
		h.Close(conn)
		return errors.New(fmt.Sprintf("proxy connection does not exists: %v <-> %v", conn.LocalAddr(), conn.RemoteAddr()))
	}
}

func (h *tcpHandler) DidSend(conn tun2socks.Connection, len uint16) {
}

func (h *tcpHandler) DidClose(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) DidAbort(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) DidReset(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) LocalDidClose(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *tcpHandler) Close(conn tun2socks.Connection) {
	h.Lock()
	defer h.Unlock()

	if rc, found := h.conns[conn]; found {
		rc.Close()
	}

	delete(h.conns, conn)
	delete(h.tgtAddrs, conn)
	delete(h.tgtSent, conn)
}
