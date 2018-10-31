// This file is copied from https://github.com/yinghuocho/gotun2socks/blob/master/udp.go

package proxy

import (
	"log"
	"sync"
	"time"

	"github.com/miekg/dns"
)

const COMMON_DNS_PORT = 53

type dnsCacheEntry struct {
	msg *dns.Msg
	exp time.Time
}

type DNSCache struct {
	servers []string
	mutex   sync.Mutex
	storage map[string]*dnsCacheEntry
}

func NewDNSCache() *DNSCache {
	cache := &DNSCache{storage: make(map[string]*dnsCacheEntry)}
	go cache.cleanUp()
	return cache
}

func packUint16(i uint16) []byte { return []byte{byte(i >> 8), byte(i)} }

func cacheKey(q dns.Question) string {
	return string(append([]byte(q.Name), packUint16(q.Qtype)...))
}

func (c *DNSCache) cleanUp() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			c.mutex.Lock()
			log.Printf("cleaning up dns cache, %v entries", len(c.storage))
			newStorage := make(map[string]*dnsCacheEntry)
			for key, entry := range c.storage {
				if time.Now().Before(entry.exp) {
					newStorage[key] = entry
				}
			}
			c.storage = newStorage
			log.Printf("cleanup done, remaining %v entries", len(c.storage))
			c.mutex.Unlock()
		}
	}
}

func (c *DNSCache) Query(payload []byte) *dns.Msg {
	request := new(dns.Msg)
	e := request.Unpack(payload)
	if e != nil {
		return nil
	}
	if len(request.Question) == 0 {
		return nil
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	key := cacheKey(request.Question[0])
	entry := c.storage[key]
	if entry == nil {
		return nil
	}
	if time.Now().After(entry.exp) {
		delete(c.storage, key)
		return nil
	}
	entry.msg.Id = request.Id
	// log.Printf("got dns answer with key: %v", key)
	return entry.msg
}

func (c *DNSCache) Store(payload []byte) {
	resp := new(dns.Msg)
	e := resp.Unpack(payload)
	if e != nil {
		return
	}
	if resp.Rcode != dns.RcodeSuccess {
		return
	}
	if len(resp.Question) == 0 || len(resp.Answer) == 0 {
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	key := cacheKey(resp.Question[0])
	c.storage[key] = &dnsCacheEntry{
		msg: resp,
		exp: time.Now().Add(time.Duration(resp.Answer[0].Header().Ttl) * time.Second),
	}
	log.Printf("stored dns answer with key: %v, ttl: %v sec", key, resp.Answer[0].Header().Ttl)
}
