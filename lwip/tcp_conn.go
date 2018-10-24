package lwip

/*
#cgo CFLAGS: -I./src/include
#include "lwip/tcp.h"
*/
import "C"
import (
	"context"
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
	aborting   bool
	ctx        context.Context
	cancel     context.CancelFunc

	// Data from remote not yet write to local will buffer into this channel.
	localWriteCh    chan []byte
	localWriteSubCh chan []byte
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

	ctx, cancel := context.WithCancel(context.Background())

	conn := &tcpConn{
		pcb:     pcb,
		handler: handler,
		network: "tcp",
		// FIXME: need to handle IPv6
		remoteAddr:      GetIP4Addr(pcb.local_ip),
		remotePort:      uint16(pcb.local_port),
		localAddr:       GetIP4Addr(pcb.remote_ip),
		localPort:       uint16(pcb.remote_port),
		connKeyArg:      connKeyArg,
		connKey:         connKey,
		closing:         false,
		aborting:        false,
		ctx:             ctx,
		cancel:          cancel,
		localWriteCh:    make(chan []byte, 64),
		localWriteSubCh: make(chan []byte, 1),
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
		return errors.New(fmt.Sprintf("conn %v <-> %v is closing", conn.LocalAddr(), conn.RemoteAddr()))
	}

	err := conn.handler.DidReceive(conn, data)
	if err != nil {
		return errors.New(fmt.Sprintf("write proxy failed: %v", err))
	}

	C.tcp_recved(conn.pcb, C.u16_t(len(data)))

	return nil
}

func (conn *tcpConn) tryWriteLocal() {
	lwipMutex.Lock()
	defer lwipMutex.Unlock()

Loop:
	for {
		// Using 2 select to ensure data in localWriteSubCh will be drained first.
		select {
		case data := <-conn.localWriteSubCh:
			written, err := conn.tcpWrite(data)
			if !written || err != nil {
				// Data not written, buffer again.
				conn.localWriteSubCh <- data
				break Loop
			}
		default:
		}

		select {
		case data := <-conn.localWriteSubCh:
			written, err := conn.tcpWrite(data)
			if !written || err != nil {
				// Data not written, buffer again.
				conn.localWriteSubCh <- data
				break Loop
			}
		case data := <-conn.localWriteCh:
			written, err := conn.tcpWrite(data)
			if !written || err != nil {
				// Data not written, buffer again.
				conn.localWriteSubCh <- data
				break Loop
			}
		default:
			break Loop
		}
	}

	// Actually send data.
	if conn.pcb == nil {
		log.Fatal("tcp_output nil pcb")
	}
	err := C.tcp_output(conn.pcb)
	if err != C.ERR_OK {
		log.Printf("tcp_output error with lwip error code: %v", int(err))
	}
}

// While calling this function, the lwIP thread is assumed to be already locked by the caller.
func (conn *tcpConn) tcpWrite(data []byte) (bool, error) {
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
		return 0, errors.New(fmt.Sprintf("conn %v <-> %v is closing", conn.LocalAddr(), conn.RemoteAddr()))
	}

	select {
	case conn.localWriteCh <- append([]byte(nil), data...):
	case <-conn.ctx.Done():
		return 0, conn.ctx.Err()
	}

	// Try to send pending data if any, and call tcp_output().
	go conn.tryWriteLocal()

	return len(data), nil
}

func (conn *tcpConn) Sent(len uint16) error {
	conn.handler.DidSend(conn, len)
	// Some packets are acknowledged by local client, check if any pending data to send.
	return conn.CheckState()
}

func (conn *tcpConn) isClosing() bool {
	conn.Lock()
	defer conn.Unlock()
	return conn.closing
}

func (conn *tcpConn) isAborting() bool {
	conn.Lock()
	defer conn.Unlock()
	return conn.aborting
}

func (conn *tcpConn) CheckState() error {
	// Still have data to send
	if len(conn.localWriteCh) > 0 {
		go conn.tryWriteLocal()
		// Return and wait for the Sent() callback to be called, and then check again.
		return NewLWIPError(LWIP_ERR_OK)
	}

	if conn.isClosing() {
		conn.closeInternal()
	}

	if conn.isAborting() {
		conn.abortInternal()
		return NewLWIPError(LWIP_ERR_ABRT)
	}

	return NewLWIPError(LWIP_ERR_OK)
}

func (conn *tcpConn) Close() error {
	conn.Lock()
	defer conn.Unlock()

	// Close maybe called outside of lwIP thread, we should not call tcp_close() in this
	// function, instead just make a flag to indicate we are closing the connection.
	conn.closing = true
	return nil
}

func (conn *tcpConn) closeInternal() error {
	if conn.pcb == nil {
		log.Fatal("nil pcb when close, maybe aborted already")
	}

	C.tcp_arg(conn.pcb, nil)
	C.tcp_recv(conn.pcb, nil)
	C.tcp_sent(conn.pcb, nil)
	C.tcp_err(conn.pcb, nil)

	conn.Release()

	conn.cancel()

	err := C.tcp_close(conn.pcb)
	if err == C.ERR_OK {
		return nil
	} else {
		return errors.New(fmt.Sprint("close TCP connection failed, lwip error code %d", int(err)))
	}
}

func (conn *tcpConn) abortInternal() {
	log.Printf("aborted TCP connection %v->%v", conn.LocalAddr(), conn.RemoteAddr())
	conn.Release()
	C.tcp_abort(conn.pcb)
}

func (conn *tcpConn) Abort() {
	conn.Lock()
	defer conn.Unlock()

	conn.aborting = true
}

// The corresponding pcb is already freed when this callback is called
func (conn *tcpConn) Err(err error) {
	log.Printf("error on TCP connection %v->%v: %v", conn.LocalAddr(), conn.RemoteAddr(), err)
	conn.Release()
	conn.handler.DidClose(conn)
}

func (conn *tcpConn) LocalDidClose() error {
	log.Printf("local close TCP connection %v->%v", conn.LocalAddr(), conn.RemoteAddr())
	conn.handler.LocalDidClose(conn)
	conn.Close()             // flag closing
	return conn.CheckState() // check pending data
}

func (conn *tcpConn) Release() {
	if _, found := tcpConns.Load(conn.connKey); found {
		FreeConnKeyArg(conn.connKeyArg)
		tcpConns.Delete(conn.connKey)
	}
	log.Printf("ended TCP connection %v->%v", conn.LocalAddr(), conn.RemoteAddr())
}
func (conn *tcpConn) Poll() error {
	return conn.CheckState()
}
