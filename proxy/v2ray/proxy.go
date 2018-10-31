package v2ray

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/miekg/dns"
	vcore "v2ray.com/core"
	vnet "v2ray.com/core/common/net"
	vsession "v2ray.com/core/common/session"
	vdns "v2ray.com/core/features/dns"

	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy"
)

func isIPv4(ip net.IP) bool {
	if ip.To4() != nil {
		return true
	}
	return false
}

func isIPv6(ip net.IP) bool {
	// To16() also valid for ipv4, ensure it's not an ipv4 address
	if ip.To4() != nil {
		return false
	}
	if ip.To16() != nil {
		return true
	}
	return false
}

type connEntry struct {
	conn                net.Conn
	target              vnet.Destination
	cancelFetchingInput context.CancelFunc
	fetchingInputCtx    context.Context
}

type dnsRespEntry struct {
	err  error
	data []byte
	conn core.Connection
}

type handler struct {
	sync.Mutex

	ctx        context.Context
	v          *vcore.Instance
	conns      map[core.Connection]*connEntry
	dispatched map[core.Connection]bool
	dnsRespCh  chan *dnsRespEntry
	dnsClient  vdns.Client
}

func (h *handler) shouldAcceptDNSQuery(data []byte) bool {
	req := new(dns.Msg)
	err := req.Unpack(data)
	if err != nil {
		return false
	}

	// TODO: allow multiple question
	if len(req.Question) != 1 {
		return false
	}

	qtype := req.Question[0].Qtype
	if qtype != dns.TypeA && qtype != dns.TypeAAAA {
		return false
	}

	fqdn := req.Question[0].Name
	domain := fqdn[:len(fqdn)-1]

	if _, ok := dns.IsDomainName(domain); !ok {
		return false
	}

	return true
}

func (h *handler) handleDNSQuery(conn core.Connection, data []byte) {
	var err error
	var answer []byte = nil
	defer func() {
		h.dnsRespCh <- &dnsRespEntry{conn: conn, data: answer, err: err}
	}()

	// No error checks here because they are already done in shouldAcceptDNSQuery()
	req := new(dns.Msg)
	req.Unpack(data)
	qtype := req.Question[0].Qtype
	fqdn := req.Question[0].Name
	domain := fqdn[:len(fqdn)-1]

	log.Printf("dispatch dns request for domain: %v", domain)
	ips, err := h.dnsClient.LookupIP(domain)
	if err != nil {
		err = errors.New(fmt.Sprintf("lookup ip failed: %v", err))
		return
	}

	resp := new(dns.Msg)
	resp.SetReply(req)
	resp.RecursionAvailable = true
	for _, ip := range ips {
		if isIPv4(ip) && qtype == dns.TypeA {
			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:     fqdn,
					Rrtype:   dns.TypeA,
					Class:    dns.ClassINET,
					Ttl:      1, // cached in V2Ray
					Rdlength: 4,
				},
				A: ip,
			})
		} else if isIPv6(ip) && qtype == dns.TypeAAAA {
			resp.Answer = append(resp.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:     fqdn,
					Rrtype:   dns.TypeAAAA,
					Class:    dns.ClassINET,
					Ttl:      1, // cached in V2Ray
					Rdlength: 16,
				},
				AAAA: ip,
			})
		}
	}
	if len(resp.Answer) == 0 {
		err = errors.New("no answer")
		return
	}
	buf := core.NewBytes(core.BufSize)
	defer core.FreeBytes(buf)
	dnsAnswer, err := resp.PackBuffer(buf)
	if err != nil {
		err = errors.New(fmt.Sprintf("packing dns resp msg failed: %v", err))
	} else {
		answer = append([]byte(nil), dnsAnswer...)
	}
	return
}

func (h *handler) handleDNSResponse() {
	for {
		select {
		case respEntry := <-h.dnsRespCh:
			if respEntry.err == nil {
				_, err := respEntry.conn.Write(respEntry.data)
				if err != nil {
					log.Printf("write dns response to local failed: %v", err)
				}
			} else {
				log.Printf("dispatch dns request failed: %v", respEntry.err)
			}
			h.Close(respEntry.conn)
			respEntry.conn.Close()
		}
	}
}

func (h *handler) fetchInput(conn core.Connection) {
	h.Lock()
	c, ok := h.conns[conn]
	h.Unlock()
	if !ok {
		return
	}

	buf := core.NewBytes(core.BufSize)

FetchingLoop:
	for {
		c.conn.SetReadDeadline(time.Now().Add(4 * time.Second))
		n, err := c.conn.Read(buf)
		if err, ok := err.(net.Error); ok && err.Timeout() {
			select {
			case <-c.fetchingInputCtx.Done():
				core.FreeBytes(buf)
				// Request was handed to V2Ray, stop fetching but leave the
				// connection open.
				return
			default:
				continue FetchingLoop
			}
		}
		if err != nil {
			// log.Printf("fetch input failed: %v", err)
			h.Close(conn)
			conn.Close()
			core.FreeBytes(buf)
			return
		}
		_, err = conn.Write(buf[:n])
		if err != nil {
			// log.Printf("write local failed: %v", err)
			h.Close(conn)
			conn.Close()
			core.FreeBytes(buf)
			return
		}
	}
}

func NewHandler(ctx context.Context, instance *vcore.Instance) core.ConnectionHandler {
	h := &handler{
		ctx:        ctx,
		v:          instance,
		conns:      make(map[core.Connection]*connEntry, 16),
		dnsRespCh:  make(chan *dnsRespEntry, 1024),
		dispatched: make(map[core.Connection]bool, 16),
		dnsClient:  instance.GetFeature(vdns.ClientType()).(vdns.Client),
	}
	go h.handleDNSResponse()
	return h
}

func (h *handler) Connect(conn core.Connection, target net.Addr) error {
	dest := vnet.DestinationFromAddr(target)
	sid := vsession.NewID()
	ctx := vsession.ContextWithID(h.ctx, sid)
	c, err := vcore.Dial(ctx, h.v, dest)
	if err != nil {
		return errors.New(fmt.Sprintf("dial V proxy connection failed: %v", err))
	}
	// Note that ctx here is used for canceling fetching input goroutine, not
	// canceling the connection, thus create the cancelable context after Dial().
	ctx, cancel := context.WithCancel(ctx)
	h.Lock()
	h.conns[conn] = &connEntry{
		conn:                c,
		target:              dest,
		cancelFetchingInput: cancel,
		fetchingInputCtx:    ctx,
	}
	h.Unlock()
	go h.fetchInput(conn)
	log.Printf("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}

func (h *handler) DidReceive(conn core.Connection, data []byte) error {
	h.Lock()
	c, ok := h.conns[conn]
	done, ok2 := h.dispatched[conn]
	h.Unlock()
	if ok2 && done {
		// Request already dispatched to V2Ray, ignore.
		return nil
	}
	if ok {
		// If it's a DNS request of type A or AAAA and has only one question,
		// handle it with V2Ray's DNS client, otherwise treat as normal TCP/UDP
		// traffic.
		if c.target.Network == vnet.Network_UDP &&
			c.target.Port.Value() == proxy.COMMON_DNS_PORT &&
			h.shouldAcceptDNSQuery(data) {

			// Parse DNS request and hand to V2Ray, upon V2Ray returns []net.IP,
			// packing them into dns.Msg response message and write back to the client.
			go h.handleDNSQuery(conn, append([]byte(nil), data...))

			// The DNS request has passed to V2Ray for handling, we are safe
			// to cancel the fetching goroutine, but be careful do not close the
			// connection.
			c.cancelFetchingInput()

			h.Lock()
			// The request is successfully handed to V2Ray, mark as dispatched so
			// subsequent retransmissions should be ignored.
			h.dispatched[conn] = true
			h.Unlock()
		} else {
			_, err := c.conn.Write(data)
			if err != nil {
				h.Close(conn)
				return errors.New(fmt.Sprintf("write remote failed: %v", err))
			}
		}
		return nil
	} else {
		h.Close(conn)
		return errors.New(fmt.Sprintf("proxy connection %v->%v does not exists", conn.LocalAddr(), conn.RemoteAddr()))
	}
}

func (h *handler) DidSend(conn core.Connection, len uint16) {
	// unused
}

func (h *handler) DidClose(conn core.Connection) {
	h.Close(conn)
}

func (h *handler) LocalDidClose(conn core.Connection) {
	h.Close(conn)
}

func (h *handler) Close(conn core.Connection) {
	h.Lock()
	defer h.Unlock()

	if c, found := h.conns[conn]; found {
		c.conn.Close()
	}
	delete(h.conns, conn)
}
