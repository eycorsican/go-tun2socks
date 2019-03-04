package dns

const COMMON_DNS_PORT = 53

type DnsCache interface {
	// Query queries the response for the DNS request with payload `p`,
	// the response data should be a valid DNS response payload.
	Query(p []byte) []byte

	// Store stores the DNS response with payload `p` to the cache.
	Store(p []byte)
}
