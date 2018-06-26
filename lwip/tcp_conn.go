package lwip

/*
#cgo CFLAGS: -I./src/include
#include "lwip/tcp.h"
*/
import "C"
import (
	"errors"
	"log"
	"math/rand"
	"net"
	"time"
	"unsafe"

	tun2socks "github.com/eycorsican/go-tun2socks"
)

var (
	tcpAbortError   = errors.New("TCP abort")
	tcpResetError   = errors.New("TCP reset")
	tcpUnknownError = errors.New("unknown TCP error")
)

type tcpConn struct {
	pcb        *C.struct_tcp_pcb
	handler    tun2socks.ConnectionHandler
	network    string
	remoteAddr string
	remotePort uint16
	localAddr  string
	localPort  uint16
	connKeyArg unsafe.Pointer
	connKey    uint32
}

func checkTCPConns() {
	tcpConns.Range(func(_, c interface{}) bool {
		state := c.(*tcpConn).pcb.state
		if c.(*tcpConn).pcb == nil ||
			state == C.CLOSED ||
			state == C.TIME_WAIT ||
			state == C.CLOSE_WAIT {
			c.(*tcpConn).Release()
		}
		return true
	})
}

func NewTCPConnection(pcb *C.struct_tcp_pcb, handler tun2socks.ConnectionHandler) tun2socks.Connection {
	// prepare key
	connKeyArg := NewConnKeyArg()
	connKey := rand.Uint32()
	SetConnKeyVal(unsafe.Pointer(connKeyArg), connKey)

	if tcpConnectionHandler == nil {
		log.Printf("no TCP connection handler found")
		return nil
	}

	conn := &tcpConn{
		pcb:     pcb,
		handler: handler,
		network: "tcp",
		// FIXME: need to handle IPv6
		remoteAddr: GetIP4Addr(pcb.local_ip),
		remotePort: uint16(pcb.local_port),
		localAddr:  GetIP4Addr(pcb.remote_ip),
		localPort:  uint16(pcb.remote_port),
		connKeyArg: connKeyArg,
		connKey:    connKey,
	}

	// Associate conn with key and save to the global map.
	tcpConns.Store(connKey, conn)

	go checkTCPConns()

	// Pass the key as arg for subsequent tcp callbacks.
	C.tcp_arg(pcb, unsafe.Pointer(connKeyArg))

	SetTCPRecvCallback(pcb)
	SetTCPSentCallback(pcb)
	SetTCPErrCallback(pcb)

	// FIXME: return and handle connect error
	handler.Connect(conn, conn.RemoteAddr())
	return conn
}

func (conn *tcpConn) RemoteAddr() net.Addr {
	return MustResolveTCPAddr(conn.remoteAddr, conn.remotePort)
}

func (conn *tcpConn) LocalAddr() net.Addr {
	return MustResolveTCPAddr(conn.localAddr, conn.localPort)
}

func (conn *tcpConn) Receive(data []byte) error {
	// Process received data
	err := conn.handler.DidReceive(conn, data)
	if err != nil {
		conn.Abort()
		return errors.New("failed to handle received TCP data")
	}

	// We should call tcp_recved() after data have been processed, by default we assume
	// all pbuf data have been processed.
	C.tcp_recved(conn.pcb, C.u16_t(len(data)))
	return nil
}

func (conn *tcpConn) Write(data []byte) error {
	lwipMutex.Lock()
	for {

		if conn.pcb == nil {
			lwipMutex.Unlock()
			return errors.New("nil tcp pcb")
		}

		err := C.tcp_write(conn.pcb, unsafe.Pointer(&data[0]), C.u16_t(len(data)), C.TCP_WRITE_FLAG_COPY)
		if err != C.ERR_OK {
			if err == C.ERR_MEM {
				lwipMutex.Unlock()
				time.Sleep(10 * time.Millisecond)
				lwipMutex.Lock()
				continue
			}
			conn.Close()
			lwipMutex.Unlock()
			return errors.New("failed to enqueue data: ERR_OTHER")
		} else {
			if conn.pcb == nil {
				lwipMutex.Unlock()
				return errors.New("nil tcp pcb")
			}
			err = C.tcp_output(conn.pcb)
			if err != C.ERR_OK {
				log.Printf("failed to output data: %v", err)
			}
		}
		lwipMutex.Unlock()
		return nil
	}
}

func (conn *tcpConn) Sent(len uint16) {
	conn.handler.DidSend(conn, len)
}

func (conn *tcpConn) Close() error {
	if conn.pcb == nil {
		log.Printf("nil pcb when close")
		return nil
	}

	C.tcp_arg(conn.pcb, nil)
	C.tcp_recv(conn.pcb, nil)
	C.tcp_sent(conn.pcb, nil)
	C.tcp_err(conn.pcb, nil)

	err := C.tcp_close(conn.pcb)
	if err != C.ERR_OK {
		return errors.New("failed to close tcp connection")
	}

	conn.Release()
	conn.handler.DidClose(conn)
	return nil
}

func (conn *tcpConn) Abort() {
	if conn.pcb == nil {
		log.Printf("nil pcb when abort")
		conn.handler.DidClose(conn)
		return
	}

	C.tcp_abort(conn.pcb)
	conn.Release()
	// conn.handler.DidClose(conn)
}

func (conn *tcpConn) Err(err error) {
	conn.Release()
	conn.handler.DidClose(conn)
}

func (conn *tcpConn) LocalDidClose() {
	conn.handler.LocalDidClose(conn)
}

func (conn *tcpConn) Reset() {
	if conn.pcb == nil {
		return
	}

	C.tcp_arg(conn.pcb, nil)
	C.tcp_recv(conn.pcb, nil)
	C.tcp_sent(conn.pcb, nil)
	C.tcp_err(conn.pcb, nil)
	C.tcp_abort(conn.pcb)
	conn.Release()
}

func (conn *tcpConn) Release() {
	conn.pcb = nil

	if _, found := tcpConns.Load(conn.connKey); found {
		FreeConnKeyArg(conn.connKeyArg)
		tcpConns.Delete(conn.connKey)
		log.Printf("released a TCP connection, total: %v", GetSyncMapLen(tcpConns))
	}
}
