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
	}
	conn, found := udpConns.Load(connId)
	if !found {
		if udpConnectionHandler == nil {
			log.Printf("no UDP connection handler found")
			return
		}
		conn = NewUDPConnection(pcb,
			udpConnectionHandler,
			*addr,
			*destAddr,
			port,
			destPort,
		)
		if conn == nil {
			log.Printf("failed to create UDP relay connection")
			return
		}
		udpConns.Store(connId, conn)
		log.Printf("created new UDP connection %v <-> %v, total: %v", conn.(tun2socks.Connection).LocalAddr().String(), conn.(tun2socks.Connection).RemoteAddr().String(), GetSyncMapLen(udpConns))
	}

	buf := NewBytes()
	C.pbuf_copy_partial(p, unsafe.Pointer(&buf[0]), p.tot_len, 0)
	conn.(tun2socks.Connection).Receive(buf[:int(p.tot_len)])
	FreeBytes(buf)
}
