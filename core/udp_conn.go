package core

/*
#cgo CFLAGS: -I./src/include
#include "lwip/udp.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"net"
	"sync"
	"unsafe"
)

type udpConn struct {
	sync.Mutex

	pcb        *C.struct_udp_pcb
	handler    ConnectionHandler
	localAddr  net.Addr
	remoteAddr net.Addr
	localIP    C.ip_addr_t
	remoteIP   C.ip_addr_t
	remotePort C.u16_t
	localPort  C.u16_t
	closed     bool
}

func newUDPConnection(pcb *C.struct_udp_pcb, handler ConnectionHandler, localIP, remoteIP C.ip_addr_t, localPort, remotePort C.u16_t) (Connection, error) {
	conn := &udpConn{
		handler:    handler,
		pcb:        pcb,
		localAddr:  ParseUDPAddr(ipAddrNTOA(localIP), uint16(localPort)),
		remoteAddr: ParseUDPAddr(ipAddrNTOA(remoteIP), uint16(remotePort)),
		localIP:    localIP,
		remoteIP:   remoteIP,
		localPort:  localPort,
		remotePort: remotePort,
		closed:     false,
	}
	err := handler.Connect(conn, conn.RemoteAddr())
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (conn *udpConn) RemoteAddr() net.Addr {
	return conn.remoteAddr
}

func (conn *udpConn) LocalAddr() net.Addr {
	return conn.localAddr
}

func (conn *udpConn) Receive(data []byte) error {
	if conn.isClosed() {
		return errors.New("connection closed")
	}
	lwipMutex.Unlock()
	err := conn.handler.DidReceive(conn, data)
	lwipMutex.Lock()
	if err != nil {
		return errors.New(fmt.Sprintf("write proxy failed: %v", err))
	}
	return nil
}

func (conn *udpConn) Write(data []byte) (int, error) {
	if conn.closed {
		return 0, errors.New("connection closed")
	}
	if conn.pcb == nil {
		return 0, errors.New("nil udp pcb")
	}
	buf := C.pbuf_alloc_reference(unsafe.Pointer(&data[0]), C.u16_t(len(data)), C.PBUF_ROM)
	C.udp_sendto(conn.pcb, buf, &conn.localIP, conn.localPort, &conn.remoteIP, conn.remotePort)
	C.pbuf_free(buf)
	return len(data), nil
}

func (conn *udpConn) Sent(len uint16) error {
	// unused
	return nil
}

func (conn *udpConn) isClosed() bool {
	conn.Lock()
	defer conn.Unlock()
	return conn.closed
}

func (conn *udpConn) Close() error {
	connId := udpConnId{
		src: conn.LocalAddr().String(),
		dst: conn.RemoteAddr().String(),
	}
	conn.Lock()
	conn.closed = true
	conn.Unlock()
	udpConns.Delete(connId)
	return nil
}

func (conn *udpConn) Err(err error) {
	// unused
}

func (conn *udpConn) Abort() {
	// unused
}

func (conn *udpConn) LocalDidClose() error {
	// unused
	return nil
}

func (conn *udpConn) Poll() error {
	// unused
	return nil
}
