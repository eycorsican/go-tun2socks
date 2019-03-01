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

type udpConnState uint

const (
	udpNewConn udpConnState = iota
	udpConnecting
	udpConnected
	udpClosed
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
	state      udpConnState
	pending    chan []byte
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
		state:      udpNewConn,
		pending:    make(chan []byte, 1), // For DNS request payload.
	}

	conn.Lock()
	conn.state = udpConnecting
	conn.Unlock()
	go func() {
		err := handler.Connect(conn, conn.RemoteAddr())
		if err != nil {
			conn.Close()
		} else {
			conn.Lock()
			conn.state = udpConnected
			conn.Unlock()
			// Once connected, send all pending data.
		DrainPending:
			for {
				select {
				case data := <-conn.pending:
					err := conn.handler.DidReceive(conn, data)
					if err != nil {
						continue DrainPending
					}
				default:
					break DrainPending
				}
			}
		}
	}()

	return conn, nil
}

func (conn *udpConn) RemoteAddr() net.Addr {
	return conn.remoteAddr
}

func (conn *udpConn) LocalAddr() net.Addr {
	return conn.localAddr
}

func (conn *udpConn) checkState() error {
	conn.Lock()
	defer conn.Unlock()

	switch conn.state {
	case udpClosed:
		return errors.New("connection closed")
	case udpConnected:
		return nil
	case udpNewConn, udpConnecting:
		return errors.New("not connected")
	}
	return nil
}

func (conn *udpConn) isConnecting() bool {
	conn.Lock()
	defer conn.Unlock()
	return conn.state == udpConnecting
}

func (conn *udpConn) Receive(data []byte) error {
	if conn.isConnecting() {
		select {
		// Data will be dropped if pending is full.
		case conn.pending <- data:
			return nil
		default:
		}
	}
	if err := conn.checkState(); err != nil {
		return err
	}
	err := conn.handler.DidReceive(conn, data)
	if err != nil {
		return errors.New(fmt.Sprintf("write proxy failed: %v", err))
	}
	return nil
}

func (conn *udpConn) Write(data []byte) (int, error) {
	if err := conn.checkState(); err != nil {
		return 0, err
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

func (conn *udpConn) Close() error {
	connId := udpConnId{
		src: conn.LocalAddr().String(),
		dst: conn.RemoteAddr().String(),
	}
	conn.Lock()
	conn.state = udpClosed
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
