package fakedns

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/miekg/dns"

	cdns "github.com/eycorsican/go-tun2socks/common/dns"
	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

type simpleFakeDns struct {
	sync.Mutex

	// TODO cleanup map
	ip2domain map[uint32]string
	cursor    uint32
}

func canHandleDnsQuery(data []byte) bool {
	req := new(dns.Msg)
	err := req.Unpack(data)
	if err != nil {
		log.Debugf("cannot handle dns query: failed to unpack")
		return false
	}
	if len(req.Question) != 1 {
		log.Debugf("cannot handle dns query: multiple questions")
		return false
	}
	qtype := req.Question[0].Qtype
	if qtype != dns.TypeA && qtype != dns.TypeAAAA {
		log.Debugf("cannot handle dns query: not A/AAAA qtype")
		return false
	}
	qclass := req.Question[0].Qclass
	if qclass != dns.ClassINET {
		log.Debugf("cannot handle dns query: not ClassINET")
		return false
	}
	fqdn := req.Question[0].Name
	domain := fqdn[:len(fqdn)-1]
	if _, ok := dns.IsDomainName(domain); !ok {
		log.Debugf("cannot handle dns query: invalid domain name")
		return false
	}
	return true
}

func uint322ip(n uint32) net.IP {
	b1 := (n & 0xff000000) >> 24
	b2 := (n & 0x00ff0000) >> 16
	b3 := (n & 0x0000ff00) >> 8
	b4 := (n & 0x000000ff)
	return net.IPv4(byte(b1), byte(b2), byte(b3), byte(b4))
}

func ip2uint32(ip net.IP) uint32 {
	return binary.BigEndian.Uint32([]byte(ip)[net.IPv6len-net.IPv4len:])
}

func NewSimpleFakeDns() cdns.FakeDns {
	return &simpleFakeDns{
		ip2domain: make(map[uint32]string, 64),
		cursor:    cdns.MinFakeIPCursor,
	}
}

func (f *simpleFakeDns) allocateIP(domain string) net.IP {
	f.Lock()
	defer f.Unlock()
	f.ip2domain[f.cursor] = domain
	ip := uint322ip(f.cursor)
	f.cursor += 1
	if f.cursor > cdns.MaxFakeIPCursor {
		f.cursor = cdns.MinFakeIPCursor
	}
	return ip
}

func (f *simpleFakeDns) QueryDomain(ip net.IP) string {
	f.Lock()
	defer f.Unlock()
	if domain, found := f.ip2domain[ip2uint32(ip)]; found {
		log.Debugf("fake dns returns domain %v for ip %v", domain, ip)
		return domain
	}
	return ""
}

func (f *simpleFakeDns) GenerateFakeResponse(request []byte) ([]byte, error) {
	if canHandleDnsQuery(request) {
		req := new(dns.Msg)
		req.Unpack(request)
		qtype := req.Question[0].Qtype
		fqdn := req.Question[0].Name
		domain := fqdn[:len(fqdn)-1]
		ip := f.allocateIP(domain)
		log.Debugf("fake dns allocated ip %v for domain %v", ip, domain)
		resp := new(dns.Msg)
		resp = resp.SetReply(req)
		if qtype == dns.TypeA {
			resp.Answer = append(resp.Answer, &dns.A{
				Hdr: dns.RR_Header{
					Name:     fqdn,
					Rrtype:   dns.TypeA,
					Class:    dns.ClassINET,
					Ttl:      1,
					Rdlength: net.IPv4len,
				},
				A: ip,
			})
		} else if qtype == dns.TypeAAAA {
			resp.Answer = append(resp.Answer, &dns.AAAA{
				Hdr: dns.RR_Header{
					Name:     fqdn,
					Rrtype:   dns.TypeAAAA,
					Class:    dns.ClassINET,
					Ttl:      1,
					Rdlength: net.IPv6len,
				},
				AAAA: ip,
			})
		} else {
			return nil, fmt.Errorf("unexcepted dns qtype %v", qtype)
		}
		buf := core.NewBytes(core.BufSize)
		defer core.FreeBytes(buf)
		dnsAnswer, err := resp.PackBuffer(buf)
		if err != nil {
			return nil, fmt.Errorf("failed to pack dns answer: %v", err)
		}
		return append([]byte(nil), dnsAnswer...), nil
	} else {
		return nil, errors.New("cannot handle DNS request")
	}
}
