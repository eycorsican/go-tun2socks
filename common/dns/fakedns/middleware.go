package fakedns

import (
	"fmt"
	"net"
	"strings"

	trie "github.com/eycorsican/go-tun2socks/common/dns/domain-trie"
	D "github.com/miekg/dns"
)

type handler func(w D.ResponseWriter, r *D.Msg)

func withFakeIP(cache *Cache, pool *Pool) handler {
	return func(w D.ResponseWriter, r *D.Msg) {
		q := r.Question[0]

		cacheItem := cache.Get("fakeip:" + q.String())
		if cacheItem != nil {
			msg := cacheItem.(*D.Msg).Copy()
			setMsgTTL(msg, dnsFakeTTL)
			msg.SetReply(r)
			_ = w.WriteMsg(msg)
			return
		}

		rr := &D.A{}
		rr.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeA, Class: D.ClassINET, Ttl: dnsDefaultTTL}
		ip := pool.Get()
		rr.A = ip
		msg := r.Copy()
		msg.Answer = []D.RR{rr}
		putMsgToCache(cache, "fakeip:"+q.String(), msg)
		putMsgToCache(cache, ip.String(), msg)

		setMsgTTL(msg, dnsFakeTTL)
		return
	}
}

func withHost(hosts *trie.Trie, next handler) handler {
	if hosts == nil {
		panic("dns/withHost: hosts should not be nil")
	}

	return func(w D.ResponseWriter, r *D.Msg) {
		q := r.Question[0]
		if q.Qtype != D.TypeA && q.Qtype != D.TypeAAAA {
			next(w, r)
			return
		}

		domain := strings.TrimRight(q.Name, ".")
		host := hosts.Search(domain)
		if host == nil {
			next(w, r)
			return
		}

		ip := host.Data.(net.IP)
		if q.Qtype == D.TypeAAAA && ip.To16() == nil {
			next(w, r)
			return
		} else if q.Qtype == D.TypeA && ip.To4() == nil {
			next(w, r)
			return
		}

		var rr D.RR
		if q.Qtype == D.TypeAAAA {
			record := &D.AAAA{}
			record.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeAAAA, Class: D.ClassINET, Ttl: dnsDefaultTTL}
			record.AAAA = ip
			rr = record
		} else {
			record := &D.A{}
			record.Hdr = D.RR_Header{Name: q.Name, Rrtype: D.TypeA, Class: D.ClassINET, Ttl: dnsDefaultTTL}
			record.A = ip
			rr = record
		}

		msg := r.Copy()
		msg.Answer = []D.RR{rr}
		msg.SetReply(r)
		_ = w.WriteMsg(msg)
		return
	}
}

func strToHosts(str string) *trie.Trie {
	if str == "" {
		return nil
	}
	tree := trie.New()
	s := strings.Split(str, ",")
	for _, host := range s {
		m := strings.Split(host, "=")
		if len(m) != 2 {
			continue
		}
		domain := strings.TrimSpace(m[0])
		target := strings.TrimSpace(m[1])
		if err := tree.Insert(domain, net.ParseIP(target)); err != nil {
			panic(fmt.Sprintf("add hosts error: %v", err))
		}
	}
	return tree
}

func newHandler(hosts *trie.Trie, cache *Cache, pool *Pool) handler {
	if hosts != nil {
		return withHost(hosts, withFakeIP(cache, pool))
	}
	return withFakeIP(cache, pool)
}
