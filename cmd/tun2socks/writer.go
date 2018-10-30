package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"time"

	vnet "v2ray.com/core/common/net"
	vsession "v2ray.com/core/common/session"
	vrouting "v2ray.com/core/features/routing"

	"github.com/eycorsican/go-tun2socks/route"
)

type icmpDelayedWriter struct {
	writer io.Writer
	delay  int
}

func (w *icmpDelayedWriter) Write(buf []byte) (int, error) {
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

type routingAwareWriter struct {
	writer  io.Writer
	router  vrouting.Router
	gateway string
}

func (w *routingAwareWriter) Write(buf []byte) (int, error) {
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

	dest, err := vnet.ParseDestination(fmt.Sprintf("%s:%s:%d", protocol, destAddr, destPort.Value()))
	if err != nil {
		return 0, errors.New(fmt.Sprintf("failed to parse destination: %v", err))
	}

	ctx := vsession.ContextWithOutbound(context.Background(), &vsession.Outbound{
		Target: dest,
	})

	tag, err := w.router.PickRoute(ctx)
	if err == nil && tag == "direct" {
		err := route.AddRoute(dest.Address.String(), "255.255.255.255", w.gateway)
		if err == nil {
			// Discarding the packet so it will bed retransmitted, and hopefully retransmitted packets will
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
