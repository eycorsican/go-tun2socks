package filter

import (
	"context"
	"io"
	"log"
	"time"

	vnet "v2ray.com/core/common/net"
	vsession "v2ray.com/core/common/session"
	vrouting "v2ray.com/core/features/routing"

	"github.com/eycorsican/go-tun2socks/route"
)

// Filter is used for filtering IP packets comming from TUN.
type Filter interface {
	io.Writer
}

type icmpFilter struct {
	writer io.Writer
	delay  int
}

func NewICMPFilter(w io.Writer, delay int) Filter {
	return &icmpFilter{writer: w, delay: delay}
}

func (w *icmpFilter) Write(buf []byte) (int, error) {
	if uint8(buf[9]) == route.PROTOCOL_ICMP {
		payload := make([]byte, len(buf))
		copy(payload, buf)
		go func(data []byte) {
			time.Sleep(time.Duration(w.delay) * time.Millisecond)
			_, err := w.writer.Write(data)
			if err != nil {
				log.Fatal("failed to input data to the stack: %v", err)
			}
		}(payload)
		return len(buf), nil
	} else {
		return w.writer.Write(buf)
	}
}

type routingFilter struct {
	writer  io.Writer
	router  vrouting.Router
	gateway string
}

func NewRoutingFilter(w io.Writer, router vrouting.Router, gateway string) Filter {
	return &routingFilter{writer: w, router: router, gateway: gateway}
}

func (w *routingFilter) Write(buf []byte) (int, error) {
	ipVersion := route.PeekIPVersion(buf)
	if ipVersion == route.IPVERSION_6 {
		// TODO No IPv6 support currently
		return w.writer.Write(buf)
	}

	if ipVersion != route.IPVERSION_4 && ipVersion != route.IPVERSION_6 {
		log.Fatal("not an IP packet: %v", buf)
	}

	protocol := route.PeekProtocol(buf)
	if protocol != "tcp" && protocol != "udp" {
		return w.writer.Write(buf)
	}
	if protocol == "tcp" && !route.IsSYNSegment(buf) {
		return w.writer.Write(buf)
	}

	destAddr := route.PeekDestinationAddress(buf)
	destPort := route.PeekDestinationPort(buf)

	var dest vnet.Destination
	switch protocol {
	case "tcp":
		dest = vnet.TCPDestination(destAddr, destPort)
	case "udp":
		dest = vnet.UDPDestination(destAddr, destPort)
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
			log.Printf("adding route for %v failed: %v", dest, err)
		}
	}

	return w.writer.Write(buf)
}
