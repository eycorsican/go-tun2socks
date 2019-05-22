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

const (
	FakeResponseTtl uint32 = 1          // in sec
)

type simpleFakeDns struct {
	sync.Mutex

	// TODO cleanup map
	ip2domain map[uint32]string

	// Cursor is an IPv4 address represent in uint32 type.
	cursor    uint32
	minCursor uint32
	maxCursor uint32

	fakeTtl uint32
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
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n))
}

func ip2uint32(ip net.IP) uint32 {
	return binary.BigEndian.Uint32([]byte(ip)[net.IPv6len-net.IPv4len:])
}

func NewSimpleFakeDns(minIP, maxIP string) cdns.FakeDns {
	parsedMinIP := net.ParseIP(minIP)
	parsedMaxIP := net.ParseIP(maxIP)
	if parsedMinIP == nil || parsedMaxIP == nil {
		return nil
	}
	minFakeIPCursor := ip2uint32(parsedMinIP)
	maxFakeIPCursor := ip2uint32(parsedMaxIP)
	return &simpleFakeDns{
		ip2domain: make(map[uint32]string, 64),
		cursor:    minFakeIPCursor,
		minCursor: minFakeIPCursor,
		maxCursor: maxFakeIPCursor,
	}
}

func (f *simpleFakeDns) allocateIP(domain string) net.IP {
	f.Lock()
	defer f.Unlock()
	f.ip2domain[f.cursor] = domain
	ip := uint322ip(f.cursor)
	f.cursor += 1
	if f.cursor > f.maxCursor {
		f.cursor = f.minCursor
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
	if !canHandleDnsQuery(request) {
		return nil, errors.New("cannot handle DNS request")
	}
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
				Ttl:      FakeResponseTtl,
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
				Ttl:      FakeResponseTtl,
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
}

func (f *simpleFakeDns) IsFakeIP(ip net.IP) bool {
	c := ip2uint32(ip)
	if c >= f.minCursor && c <= f.maxCursor {
		return true
	}
	return false
}
