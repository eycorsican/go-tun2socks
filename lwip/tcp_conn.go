package lwip

/*
#cgo CFLAGS: -I./src/include
#include "lwip/tcp.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"unsafe"

	tun2socks "github.com/eycorsican/go-tun2socks"
)

type tcpConn struct {
	sync.Mutex

	pcb        *C.struct_tcp_pcb
	handler    tun2socks.ConnectionHandler
	network    string
	remoteAddr string
	remotePort uint16
	localAddr  string
	localPort  uint16
	connKeyArg unsafe.Pointer
	connKey    uint32
	closing    bool
	localLock  sync.RWMutex

	// Data from remote not yet write to local will buffer into this channel.
	localWriteCh chan []byte
}

func checkTCPConns() {
	tcpConns.Range(func(_, c interface{}) bool {
		state := c.(*tcpConn).pcb.state
		if c.(*tcpConn).pcb == nil ||
			state == C.CLOSED ||
			state == C.CLOSE_WAIT {
			c.(*tcpConn).Release()
		}
		return true
	})
}

func NewTCPConnection(pcb *C.struct_tcp_pcb, handler tun2socks.ConnectionHandler) (tun2socks.Connection, error) {
	// prepare key
	connKeyArg := NewConnKeyArg()
	connKey := rand.Uint32()
	SetConnKeyVal(unsafe.Pointer(connKeyArg), connKey)

	if tcpConnectionHandler == nil {
		return nil, errors.New("no registered TCP connection handlers found")
	}

	conn := &tcpConn{
		pcb:     pcb,
		handler: handler,
		network: "tcp",
		// FIXME: need to handle IPv6
		remoteAddr:   GetIP4Addr(pcb.local_ip),
		remotePort:   uint16(pcb.local_port),
		localAddr:    GetIP4Addr(pcb.remote_ip),
		localPort:    uint16(pcb.remote_port),
		connKeyArg:   connKeyArg,
		connKey:      connKey,
		closing:      false,
		localWriteCh: make(chan []byte, 256),
	}

	// Associate conn with key and save to the global map.
	tcpConns.Store(connKey, conn)

	go checkTCPConns()

	// Pass the key as arg for subsequent tcp callbacks.
	C.tcp_arg(pcb, unsafe.Pointer(connKeyArg))

	SetTCPRecvCallback(pcb)
	SetTCPSentCallback(pcb)
	SetTCPErrCallback(pcb)

	err := handler.Connect(conn, conn.RemoteAddr())
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (conn *tcpConn) RemoteAddr() net.Addr {
	return MustResolveTCPAddr(conn.remoteAddr, conn.remotePort)
}

func (conn *tcpConn) LocalAddr() net.Addr {
	return MustResolveTCPAddr(conn.localAddr, conn.localPort)
}

func (conn *tcpConn) Receive(data []byte) error {
	if conn.isClosing() {
		return errors.New("conn is closing")
	}

	err := conn.handler.DidReceive(conn, data)
	if err != nil {
		return errors.New(fmt.Sprintf("write proxy failed: %v", err))
	}

	C.tcp_recved(conn.pcb, C.u16_t(len(data)))

	return nil
}

func (conn *tcpConn) tryWriteLocal() {
Loop:
	for {
		select {
		case data := <-conn.localWriteCh:
			written, err := conn.tcpWrite(data)
			if !written || err != nil {
				// Data not written, buffer again.
				conn.localWriteCh <- data
				break Loop
			}
		default:
			break Loop
		}
	}

	// Actually send data.
	lwipMutex.Lock()
	err := C.tcp_output(conn.pcb)
	lwipMutex.Unlock()
	if err != C.ERR_OK {
		log.Fatal("tcp_output error")
	}
}

func (conn *tcpConn) tcpWrite(data []byte) (bool, error) {
	lwipMutex.Lock()
	defer lwipMutex.Unlock()

	if len(data) <= int(conn.pcb.snd_buf) {
		// enqueue data
		err := C.tcp_write(conn.pcb, unsafe.Pointer(&data[0]), C.u16_t(len(data)), C.TCP_WRITE_FLAG_COPY)
		if err == C.ERR_OK {
			return true, nil
		} else if err != C.ERR_MEM {
			return false, errors.New(fmt.Sprintf("lwip tcp_write failed with error code: %v", int(err)))
		}
	}
	return false, nil
}

func (conn *tcpConn) Write(data []byte) (int, error) {
	if conn.isClosing() {
		return 0, errors.New("conn is closing")
	}

	written, err := conn.tcpWrite(data)
	if err != nil {
		return 0, err
	}

	if !written {
		// Not written yet, buffer the data.
		conn.localWriteCh <- data
	}

	// Try to send pending data if any, and call tcp_output().
	go conn.tryWriteLocal()

	return len(data), nil
}

func (conn *tcpConn) Sent(len uint16) {
	conn.handler.DidSend(conn, len)
	// Some packets are acknowledged by local client, check if any pending data to send.
	conn.CheckState()
}

func (conn *tcpConn) isClosing() bool {
	conn.Lock()
	defer conn.Unlock()
	return conn.closing
}

func (conn *tcpConn) CheckState() {
	// Still have data to send
	if len(conn.localWriteCh) > 0 {
		go conn.tryWriteLocal()
		// Return and wait for the Sent() callback to be called, and then check again.
		return
	}

	if conn.isClosing() {
		conn._close()
	}
}

func (conn *tcpConn) Close() error {
	conn.Lock()
	defer conn.Unlock()

	// Close maybe called outside of lwIP thread, we should not call tcp_close() in this
	// function, instead just make a flag to indicate we are closing the connection.
	conn.closing = true
	return nil
}

func (conn *tcpConn) _close() error {
	if conn.pcb == nil {
		log.Fatal("nil pcb when close, maybe aborted already")
	}

	C.tcp_arg(conn.pcb, nil)
	C.tcp_recv(conn.pcb, nil)
	C.tcp_sent(conn.pcb, nil)
	C.tcp_err(conn.pcb, nil)

	conn.Release()

	C.tcp_close(conn.pcb)

	return nil
}

func (conn *tcpConn) Abort() {
	conn.Release()
	C.tcp_abort(conn.pcb)
}

func (conn *tcpConn) Err(err error) {
	conn.Release()
	conn.handler.DidClose(conn)
}

func (conn *tcpConn) LocalDidClose() {
	conn.handler.LocalDidClose(conn)
	conn.Close()      // flag closing
	conn.CheckState() // check pending data
}

func (conn *tcpConn) Release() {
	if _, found := tcpConns.Load(conn.connKey); found {
		FreeConnKeyArg(conn.connKeyArg)
		tcpConns.Delete(conn.connKey)
	}
}
func (conn *tcpConn) Poll() {
	conn.CheckState()
}
