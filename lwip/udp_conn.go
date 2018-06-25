package lwip

/*
#cgo CFLAGS: -I./include
#include "lwip/udp.h"
*/
import "C"
import (
	"errors"
	"net"
	"unsafe"

	tun2socks "github.com/eycorsican/go-tun2socks"
)

type udpConn struct {
	pcb        *C.struct_udp_pcb
	handler    tun2socks.ConnectionHandler
	remoteAddr C.ip_addr_t
	localAddr  C.ip_addr_t
	remotePort C.u16_t
	localPort  C.u16_t
}

func NewUDPConnection(pcb *C.struct_udp_pcb, handler tun2socks.ConnectionHandler, localAddr, remoteAddr C.ip_addr_t, localPort, remotePort C.u16_t) tun2socks.Connection {
	conn := &udpConn{
		handler:    handler,
		pcb:        pcb,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
		localPort:  localPort,
		remotePort: remotePort,
	}
	// FIXME: return and handle connect error
	handler.Connect(conn, conn.RemoteAddr())
	return conn
}

func (conn *udpConn) RemoteAddr() net.Addr {
	return MustResolveUDPAddr(GetIP4Addr(conn.remoteAddr), uint16(conn.remotePort))
}

func (conn *udpConn) LocalAddr() net.Addr {
	return MustResolveUDPAddr(GetIP4Addr(conn.localAddr), uint16(conn.localPort))
}

func (conn *udpConn) Receive(data []byte) error {
	err := conn.handler.DidReceive(conn, data)
	if err != nil {
		return errors.New("failed to handle received UDP data")
	}
	return nil
}

func (conn *udpConn) Write(data []byte) error {
	if conn.pcb == nil {
		return errors.New("nil udp pcb")
	}

	buf := C.pbuf_alloc(C.PBUF_TRANSPORT, C.u16_t(len(data)), C.PBUF_RAM)
	C.pbuf_take(buf, unsafe.Pointer(&data[0]), C.u16_t(len(data)))
	lwipMutex.Lock()
	C.udp_sendto(conn.pcb, buf, &conn.localAddr, conn.localPort, &conn.remoteAddr, conn.remotePort)
	lwipMutex.Unlock()
	C.pbuf_free(buf)
	return nil
}

func (conn *udpConn) Sent(len uint16) {
	// unused
}

func (conn *udpConn) Close() error {
	connId := udpConnId{
		src: conn.LocalAddr().String(),
	}
	udpConns.Delete(connId)
	return nil
}

func (conn *udpConn) Abort() {
	// unused
}

func (conn *udpConn) Err(err error) {
	// unused
}

func (conn *udpConn) Reset() {
	// unused
}

func (conn *udpConn) LocalDidClose() {
	// unused
}
