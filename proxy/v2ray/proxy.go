package v2ray

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
	vsession "v2ray.com/core/common/session"

	"github.com/eycorsican/go-tun2socks/lwip"
	"github.com/eycorsican/go-tun2socks/proxy"
)

type connEntry struct {
	conn   net.Conn
	target vnet.Destination
}

type handler struct {
	sync.Mutex

	ctx      context.Context
	v        *vcore.Instance
	conns    map[lwip.Connection]*connEntry
	dnsCache *proxy.DNSCache
}

func (h *handler) fetchInput(conn lwip.Connection) {
	defer func() {
		h.Close(conn)
		conn.Close() // also close tun2socks connection here
	}()

	h.Lock()
	c, ok := h.conns[conn]
	h.Unlock()
	if !ok {
		return
	}

	// Seems a DNS response, cache it
	if c.target.Network == vnet.Network_UDP && c.target.Port.Value() == proxy.COMMON_DNS_PORT {
		buf := lwip.NewBytes(lwip.BufSize)
		defer lwip.FreeBytes(buf)
		for {
			n, err := c.conn.Read(buf)
			if err != nil {
				log.Printf("fetch input failed: %v", err)
				return
			}
			_, err = conn.Write(buf[:n])
			if err != nil {
				log.Printf("write local failed: %v", err)
				return
			}
			h.dnsCache.Store(buf[:n])
			return // DNS responses
		}
	} else {
		_, err := io.Copy(conn, c.conn)
		if err != nil {
			log.Printf("fetch input failed: %v", err)
		}
	}
}

func NewHandler(ctx context.Context, instance *vcore.Instance) lwip.ConnectionHandler {
	return &handler{
		ctx:      ctx,
		v:        instance,
		conns:    make(map[lwip.Connection]*connEntry, 16),
		dnsCache: proxy.NewDNSCache(),
	}
}

func (h *handler) Connect(conn lwip.Connection, target net.Addr) error {
	dest := vnet.DestinationFromAddr(target)
	sid := vsession.NewID()
	ctx := vsession.ContextWithID(h.ctx, sid)
	c, err := vcore.Dial(ctx, h.v, dest)
	if err != nil {
		return errors.New(fmt.Sprintf("dial V proxy connection failed: %v", err))
	}
	h.Lock()
	h.conns[conn] = &connEntry{conn: c, target: dest}
	h.Unlock()
	go h.fetchInput(conn)
	return nil
}

func (h *handler) DidReceive(conn lwip.Connection, data []byte) error {
	h.Lock()
	c, ok := h.conns[conn]
	h.Unlock()
	if ok {
		// Seems a DNS request, try to find the record in the cache first.
		if c.target.Network == vnet.Network_UDP && c.target.Port.Value() == proxy.COMMON_DNS_PORT {
			if answer := h.dnsCache.Query(data); answer != nil {
				var buf [1024]byte
				if dnsAnswer, err := answer.PackBuffer(buf[:]); err == nil {
					_, err = conn.Write(dnsAnswer)
					if err != nil {
						return errors.New(fmt.Sprintf("cache dns answer failed: %v", err))
					}
					h.Close(conn)
					conn.Close() // also close tun2socks connection here
					return nil
				}
			}
		}

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

func (h *handler) DidSend(conn lwip.Connection, len uint16) {
	// unused
}

func (h *handler) DidClose(conn lwip.Connection) {
	h.Close(conn)
}

func (h *handler) LocalDidClose(conn lwip.Connection) {
	h.Close(conn)
}

func (h *handler) Close(conn lwip.Connection) {
	h.Lock()
	defer h.Unlock()

	if c, found := h.conns[conn]; found {
		c.conn.Close()
	}
	delete(h.conns, conn)
}
