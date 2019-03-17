package dns

import (
	"encoding/binary"
	"net"
)

const COMMON_DNS_PORT = 53

type DnsCache interface {
	// Query queries the response for the DNS request with payload `p`,
	// the response data should be a valid DNS response payload.
	Query(p []byte) []byte

	// Store stores the DNS response with payload `p` to the cache.
	Store(p []byte)
}

type FakeDns interface {
	// GenerateFakeResponse generates a response for the specify request with a fake IP address.
	GenerateFakeResponse(request []byte) ([]byte, error)

	// QueryDomain returns the corresponding domain for IP.
	QueryDomain(ip net.IP) string
}

const (
	// We set fake dns response ttl to 1, 256 fake ips should be suffice.
	MinFakeIPCursor = 4043309056 // 241.0.0.0
	MaxFakeIPCursor = 4043309311 // 241.0.0.255
)

func IsFakeIP(ip net.IP) bool {
	n := binary.BigEndian.Uint32([]byte(ip)[net.IPv6len-net.IPv4len:])
	if n >= MinFakeIPCursor && n <= MaxFakeIPCursor {
		return true
	}
	return false
}
