package core

/*
#cgo CFLAGS: -I./src/include
#include "lwip/udp.h"
*/
import "C"
import (
	"unsafe"
)

//export udpRecvFn
func udpRecvFn(arg unsafe.Pointer, pcb *C.struct_udp_pcb, p *C.struct_pbuf, addr *C.ip_addr_t, port C.u16_t, destAddr *C.ip_addr_t, destPort C.u16_t) {
	defer func() {
		if p != nil {
			C.pbuf_free(p)
		}
	}()

	if pcb == nil {
		return
	}

	srcAddr := ParseUDPAddr(ipAddrNTOA(*addr), uint16(port))
	dstAddr := ParseUDPAddr(ipAddrNTOA(*destAddr), uint16(destPort))
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
			panic("must register a UDP connection handler")
		}
		var err error
		conn, err = newUDPConnection(pcb,
			udpConnectionHandler,
			*addr,
			*destAddr,
			port,
			destPort)
		if err != nil {
			return
		}
		udpConns.Store(connId, conn)
	}

	buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:int(p.tot_len):int(p.tot_len)]
	conn.(Connection).Receive(buf)
}
