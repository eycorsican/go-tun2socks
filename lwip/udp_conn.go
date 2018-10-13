package lwip

/*
#cgo CFLAGS: -I./src/include
#include "lwip/udp.h"
*/
import "C"
import (
	"errors"
	"fmt"
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

func NewUDPConnection(pcb *C.struct_udp_pcb, handler tun2socks.ConnectionHandler, localAddr, remoteAddr C.ip_addr_t, localPort, remotePort C.u16_t) (tun2socks.Connection, error) {
	conn := &udpConn{
		handler:    handler,
		pcb:        pcb,
		localAddr:  localAddr,
		remoteAddr: remoteAddr,
		localPort:  localPort,
		remotePort: remotePort,
	}
	err := handler.Connect(conn, conn.RemoteAddr())
	if err != nil {
		return nil, err
	}
	return conn, nil
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
		return errors.New(fmt.Sprintf("write proxy failed: %v", err))
	}
	return nil
}

func (conn *udpConn) Write(data []byte) (int, error) {
	if conn.pcb == nil {
		return 0, errors.New("nil udp pcb")
	}

	buf := C.pbuf_alloc(C.PBUF_TRANSPORT, C.u16_t(len(data)), C.PBUF_RAM)
	C.pbuf_take(buf, unsafe.Pointer(&data[0]), C.u16_t(len(data)))
	C.udp_sendto(conn.pcb, buf, &conn.localAddr, conn.localPort, &conn.remoteAddr, conn.remotePort)
	C.pbuf_free(buf)
	return len(data), nil
}

func (conn *udpConn) Sent(len uint16) {
	// unused
}

func (conn *udpConn) Close() error {
	connId := udpConnId{
		src: conn.LocalAddr().String(),
		dst: conn.RemoteAddr().String(),
	}
	udpConns.Delete(connId)
	return nil
}

func (conn *udpConn) Err(err error) {
	// unused
}

func (conn *udpConn) Abort() {
	// unused
}

func (conn *udpConn) LocalDidClose() {
	// unused
}

func (conn *udpConn) Poll() {
	// unused
}
