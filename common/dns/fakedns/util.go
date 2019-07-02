package fakedns

import (
	"time"

	D "github.com/miekg/dns"
)

func putMsgToCache(c *Cache, key string, msg *D.Msg) {
	var ttl time.Duration
	if len(msg.Answer) != 0 {
		ttl = time.Duration(msg.Answer[0].Header().Ttl) * time.Second
	} else if len(msg.Ns) != 0 {
		ttl = time.Duration(msg.Ns[0].Header().Ttl) * time.Second
	} else if len(msg.Extra) != 0 {
		ttl = time.Duration(msg.Extra[0].Header().Ttl) * time.Second
	} else {
		return
	}

	c.Put(key, msg.Copy(), ttl)
}

func setMsgTTL(msg *D.Msg, ttl uint32) {
	for _, answer := range msg.Answer {
		answer.Header().Ttl = ttl
	}

	for _, ns := range msg.Ns {
		ns.Header().Ttl = ttl
	}

	for _, extra := range msg.Extra {
		extra.Header().Ttl = ttl
	}
}
