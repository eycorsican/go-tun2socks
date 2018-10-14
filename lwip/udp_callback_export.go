package lwip

/*
#cgo CFLAGS: -I./src/include
#include "lwip/udp.h"
*/
import "C"
import (
	"log"
	"unsafe"

	tun2socks "github.com/eycorsican/go-tun2socks"
)

//export UDPRecvFn
func UDPRecvFn(arg unsafe.Pointer, pcb *C.struct_udp_pcb, p *C.struct_pbuf, addr *C.ip_addr_t, port C.u16_t, destAddr *C.ip_addr_t, destPort C.u16_t) {
	defer func() {
		if p != nil {
			C.pbuf_free(p)
		}
	}()

	if pcb == nil {
		log.Printf("nil udp pcb in recv fn")
		return
	}

	connId := udpConnId{
		src: MustResolveUDPAddr(GetIP4Addr(*addr), uint16(port)).String(),
		dst: MustResolveUDPAddr(GetIP4Addr(*destAddr), uint16(destPort)).String(),
	}
	conn, found := udpConns.Load(connId)
	if !found {
		if udpConnectionHandler == nil {
			log.Printf("no registered UDP connection handlers found")
			return
		}
		var err error
		conn, err = NewUDPConnection(pcb,
			udpConnectionHandler,
			*addr,
			*destAddr,
			port,
			destPort)
		if err != nil {
			log.Printf("failed to create UDP connection: %v", err)
			return
		}
		udpConns.Store(connId, conn)
		log.Printf("created new UDP connection %v <-> %v", conn.(tun2socks.Connection).LocalAddr().String(), conn.(tun2socks.Connection).RemoteAddr().String())
	}

	// TODO: p.tot_len != p.len, have multiple pbuf in the chain?
	if p.tot_len != p.len {
		log.Fatal("udp_recv p.tot_len != p.len (%v != %v)", p.tot_len, p.len)
	}

	buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:int(p.tot_len):int(p.tot_len)]
	conn.(tun2socks.Connection).Receive(buf)
}
