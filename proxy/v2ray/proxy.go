package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"sync"

	vcore "v2ray.com/core"
	vnet "v2ray.com/core/common/net"

	tun2socks "github.com/eycorsican/go-tun2socks"
)

type handler struct {
	sync.Mutex

	v     *vcore.Instance
	conns map[tun2socks.Connection]net.Conn
}

func (h *handler) fetchInput(conn tun2socks.Connection, input io.Reader) {
	defer func() {
		h.Close(conn)
		conn.Close() // also close tun2socks connection here
	}()

	_, err := io.Copy(conn.(io.Writer), input)
	if err != nil {
		log.Printf("fetch input failed: %v", err)
	}
}

func NewHandler(configFormat string, configBytes []byte) tun2socks.ConnectionHandler {
	v, err := vcore.StartInstance(configFormat, configBytes)
	if err != nil {
		log.Fatal("start V instance failed: %v", err)
	}
	return &handler{
		v:     v,
		conns: make(map[tun2socks.Connection]net.Conn, 16),
	}
}

func (h *handler) Connect(conn tun2socks.Connection, target net.Addr) error {
	ctx := context.Background()
	c, err := vcore.Dial(ctx, h.v, vnet.DestinationFromAddr(target))
	if err != nil {
		return errors.New(fmt.Sprintf("dial V proxy connection failed: %v", err))
	}
	h.Lock()
	h.conns[conn] = c
	h.Unlock()
	go h.fetchInput(conn, c)
	return nil
}

func (h *handler) DidReceive(conn tun2socks.Connection, data []byte) error {
	h.Lock()
	c, ok := h.conns[conn]
	h.Unlock()
	if ok {
		_, err := c.Write(data)
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

func (h *handler) DidSend(conn tun2socks.Connection, len uint16) {
	// unused
}

func (h *handler) DidClose(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *handler) DidAbort(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *handler) DidReset(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *handler) LocalDidClose(conn tun2socks.Connection) {
	h.Close(conn)
}

func (h *handler) Close(conn tun2socks.Connection) {
	h.Lock()
	defer h.Unlock()

	if c, found := h.conns[conn]; found {
		c.Close()
	}
	delete(h.conns, conn)
}
