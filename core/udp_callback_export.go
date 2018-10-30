package core

/*
#cgo CFLAGS: -I./src/include
#include "lwip/udp.h"
*/
import "C"
import (
	// "log"
	"unsafe"
)

//export UDPRecvFn
func UDPRecvFn(arg unsafe.Pointer, pcb *C.struct_udp_pcb, p *C.struct_pbuf, addr *C.ip_addr_t, port C.u16_t, destAddr *C.ip_addr_t, destPort C.u16_t) {
	defer func() {
		if p != nil {
			C.pbuf_free(p)
		}
	}()

	if pcb == nil {
		// log.Printf("udp_recv pcb is nil")
		return
	}

	srcAddr := ParseUDPAddr(IPAddrNTOA(*addr), uint16(port))
	dstAddr := ParseUDPAddr(IPAddrNTOA(*destAddr), uint16(destPort))
	if srcAddr == nil || dstAddr == nil {
		panic("invalid UDP address")
	}

	connId := udpConnId{
		src: srcAddr.String(),
		dst: dstAddr.String(),
	}
	conn, found := udpConns.Load(connId)
	if !found {
		if udpConnectionHandler == nil {
			panic("no registered UDP connection handlers found")
		}
		var err error
		conn, err = NewUDPConnection(pcb,
			udpConnectionHandler,
			*addr,
			*destAddr,
			port,
			destPort)
		if err != nil {
			// log.Printf("failed to create UDP connection %v:%v->%v:%v: %v", srcAddr, dstAddr, err)
			return
		}
		udpConns.Store(connId, conn)
		// log.Printf("new UDP connection %v->%v", conn.(Connection).LocalAddr(), conn.(Connection).RemoteAddr())
	}

	// TODO: p.tot_len != p.len, have multiple pbuf in the chain?
	// if p.tot_len != p.len {
	// 	log.Fatal("udp_recv p.tot_len != p.len (%v != %v)", p.tot_len, p.len)
	// }

	buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:int(p.tot_len):int(p.tot_len)]
	conn.(Connection).Receive(buf)
}
