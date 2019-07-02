package fakedns

import "C"
import (
	"errors"
	"net"
	"strings"
	"time"

	"github.com/miekg/dns"
	D "github.com/miekg/dns"
)

var (
	cacheDuration = 60 * time.Second

	dnsFakeTTL    uint32 = 1
	dnsDefaultTTL uint32 = 600
)

type Server struct {
	*D.Server
	a string
	c *Cache
	p *Pool
}

func (s *Server) ServeDNS(w D.ResponseWriter, r *D.Msg) {
	msg, err := s.handleFakeIP(r)
	if err != nil {
		D.HandleFailed(w, r)
		return
	}
	msg.SetReply(r)
	_ = w.WriteMsg(msg)
	return

}

func (s *Server) handleFakeIP(r *D.Msg) (msg *D.Msg, err error) {
	if len(r.Question) == 0 {
		err = errors.New("should have one question at least")
		return
	}

	q := r.Question[0]

	c := s.c.Get("fakeip:" + q.String())
	if c != nil {
		msg = c.(*D.Msg).Copy()
		setMsgTTL(msg, dnsFakeTTL)
		return
	}

	var ip net.IP
	defer func() {
		if msg == nil {
			return
		}

		putMsgToCache(s.c, "fakeip:"+q.String(), msg)
		putMsgToCache(s.c, ip.String(), msg)

		setMsgTTL(msg, dnsFakeTTL)
	}()

	rr := &D.A{}
	rr.Hdr = dns.RR_Header{Name: r.Question[0].Name, Rrtype: dns.TypeA, Class: dns.ClassINET, Ttl: dnsDefaultTTL}
	ip = s.p.Get()
	rr.A = ip
	msg = r.Copy()
	msg.Answer = []D.RR{rr}
	return
}

func (s *Server) StartServer() error {
	var addr = s.a
	_, port, err := net.SplitHostPort(addr)
	if port == "0" || port == "" || err != nil {
		return errors.New("address format error")
	}

	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return err
	}

	p, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return err
	}

	s.Server = &D.Server{Addr: addr, PacketConn: p, Handler: s}

	go func() {
		_ = s.ActivateAndServe()
	}()
	return nil
}

/*
// IPToHost return fake-ip
func (s *Server) IPToHost(ip net.IP) (string,bool) {
	c := s.c.Get(ip.String())
	if c == nil {
		return "", false
	}
	fqdn := c.(*D.Msg).Question[0].Name
	return strings.TrimRight(fqdn, "."),true
}
*/

func (s *Server) IPToHost(ip net.IP) string {
	c := s.c.Get(ip.String())
	if c == nil {
		return ""
	}
	fqdn := c.(*D.Msg).Question[0].Name
	return strings.TrimRight(fqdn, ".")
}

func (s *Server) IsFakeIP(ip net.IP) bool {
	c := ipToUint(ip)
	if c >= s.p.min && c <= s.p.max {
		return true
	}
	return false
}

func NewServer(addr, fakeIPRange string) (*Server, error) {
	_, ipnet, err := net.ParseCIDR(fakeIPRange)
	if err != nil {
		return nil, err
	}
	pool, err := NewPool(ipnet)
	if err != nil {
		return nil, err
	}

	return &Server{
		a: addr,
		c: NewCache(cacheDuration),
		p: pool,
	}, nil
}
