package v2ray

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	vcore "v2ray.com/core"
	vnet "v2ray.com/core/common/net"
	vsession "v2ray.com/core/common/session"

	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

type tcpConnEntry struct {
	conn net.Conn
}

type tcpHandler struct {
	sync.Mutex

	ctx   context.Context
	v     *vcore.Instance
	conns map[core.TCPConn]*tcpConnEntry
}

func (h *tcpHandler) fetchInput(conn core.TCPConn) {
	h.Lock()
	c, ok := h.conns[conn]
	h.Unlock()
	if !ok {
		return
	}

	buf := core.NewBytes(core.BufSize)
	defer core.FreeBytes(buf)

	for {
		n, err := c.conn.Read(buf)
		if err != nil && n <= 0 {
			h.Close(conn)
			conn.Close()
			return
		}
		_, err = conn.Write(buf[:n])
		if err != nil {
			h.Close(conn)
			conn.Close()
			return
		}
	}
}

func NewTCPHandler(ctx context.Context, instance *vcore.Instance) core.TCPConnHandler {
	return &tcpHandler{
		ctx:   ctx,
		v:     instance,
		conns: make(map[core.TCPConn]*tcpConnEntry, 16),
	}
}

func (h *tcpHandler) Connect(conn core.TCPConn, target net.Addr) error {
	dest := vnet.DestinationFromAddr(target)
	sid := vsession.NewID()
	ctx := vsession.ContextWithID(h.ctx, sid)
	c, err := vcore.Dial(ctx, h.v, dest)
	if err != nil {
		return errors.New(fmt.Sprintf("dial V proxy connection failed: %v", err))
	}
	h.Lock()
	h.conns[conn] = &tcpConnEntry{
		conn: c,
	}
	h.Unlock()
	go h.fetchInput(conn)
	log.Infof("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}

func (h *tcpHandler) DidReceive(conn core.TCPConn, data []byte) error {
	h.Lock()
	c, ok := h.conns[conn]
	h.Unlock()

	if ok {
		_, err := c.conn.Write(data)
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
		c.conn.Close()
	}
	delete(h.conns, conn)
}
