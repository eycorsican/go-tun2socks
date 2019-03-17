package dnsfallback

import (
	"errors"
	"net"

	"github.com/eycorsican/go-tun2socks/common/dns"
	"github.com/eycorsican/go-tun2socks/core"
)

// UDP handler that intercepts DNS queries and replies with a truncated response (TC bit)
// in order for the client to retry over TCP. This DNS/TCP fallback mechanism is
// useful for proxy servers that do not support UDP.
// Note that non-DNS UDP traffic is dropped.
type udpHandler struct{}

const (
	dnsHeaderLength = 12
	dnsMaskQr       = uint8(0x80)
	dnsMaskTc       = uint8(0x02)
	dnsMaskRcode    = uint8(0x0F)
)

func NewUDPHandler() core.UDPConnHandler {
	return &udpHandler{}
}

func (h *udpHandler) Connect(conn core.UDPConn, target net.Addr) error {
	udpAddr, ok := target.(*net.UDPAddr)
	if !ok {
		return errors.New("Unsupported address type")
	}
	if udpAddr.Port != dns.COMMON_DNS_PORT {
		return errors.New("Cannot handle non-DNS packet")
	}
	return nil
}

func (h *udpHandler) DidReceiveTo(conn core.UDPConn, data []byte, addr net.Addr) error {
	if len(data) < dnsHeaderLength {
		return errors.New("Received malformed DNS query")
	}
	//  DNS Header
	//  0  1  2  3  4  5  6  7  0  1  2  3  4  5  6  7
	//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	//  |                      ID                       |
	//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	//  |QR|   Opcode  |AA|TC|RD|RA|   Z    |   RCODE   |
	//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	//  |                    QDCOUNT                    |
	//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	//  |                    ANCOUNT                    |
	//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	//  |                    NSCOUNT                    |
	//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	//  |                    ARCOUNT                    |
	//  +--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+--+
	// Set response and truncated bits
	data[2] |= dnsMaskQr | dnsMaskTc
	// Set response code to 'no error'.
	data[3] &= ^dnsMaskRcode
	_, err := conn.WriteFrom(data, addr)
	return err
}
