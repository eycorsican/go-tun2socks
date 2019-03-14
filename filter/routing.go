// +build v2ray

package filter

import (
	"context"
	"io"

	vnet "v2ray.com/core/common/net"
	vsession "v2ray.com/core/common/session"
	vrouting "v2ray.com/core/features/routing"

	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/common/packet"
	"github.com/eycorsican/go-tun2socks/common/route"
)

type routingFilter struct {
	writer  io.Writer
	router  vrouting.Router
	gateway string
}

func NewRoutingFilter(w io.Writer, router vrouting.Router, gateway string) Filter {
	return &routingFilter{writer: w, router: router, gateway: gateway}
}

func (w *routingFilter) Write(buf []byte) (int, error) {
	ipVersion := packet.PeekIPVersion(buf)
	if ipVersion == packet.IPVERSION_6 {
		// TODO No IPv6 support currently
		return w.writer.Write(buf)
	}

	if ipVersion != packet.IPVERSION_4 && ipVersion != packet.IPVERSION_6 {
		log.Fatalf("not an IP packet: %v", buf)
	}

	protocol := packet.PeekProtocol(buf)
	if protocol != "tcp" && protocol != "udp" {
		return w.writer.Write(buf)
	}
	if protocol == "tcp" && !packet.IsSYNSegment(buf) {
		return w.writer.Write(buf)
	}

	destAddr := packet.PeekDestinationAddress(buf)
	destPort := packet.PeekDestinationPort(buf)

	var dest vnet.Destination
	switch protocol {
	case "tcp":
		p, _ := vnet.PortFromInt(uint32(destPort))
		dest = vnet.TCPDestination(vnet.IPAddress(destAddr), p)
	case "udp":
		p, _ := vnet.PortFromInt(uint32(destPort))
		dest = vnet.UDPDestination(vnet.IPAddress(destAddr), p)
	default:
		panic("invalid protocol")
	}

	ctx := vsession.ContextWithOutbound(context.Background(), &vsession.Outbound{
		Target: dest,
	})

	tag, err := w.router.PickRoute(ctx)
	if err == nil && tag == "direct" {
		err := route.AddRoute(dest.Address.String(), "255.255.255.255", w.gateway)
		if err == nil {
			// Discarding the packet so it will be retransmitted, and hopefully retransmitted packets will
			// use the new route.
			//
			// TODO: On macOS, it appears that even though the route is added to the routing table, subsequent
			// retransmitted packets will continue using the old routing policy. Local client must create a new
			// socket for utilizing the new route. Other platforms are not tested.
			// Maybe this helps: https://www.unix.com/man-page/osx/4/route/
			// log.Printf("added a direct route for destination %v, packets need re-routing, dropped", dest)
			return len(buf), nil
		} else {
			log.Warnf("adding route for %v failed: %v", dest, err)
		}
	}

	return w.writer.Write(buf)
}
