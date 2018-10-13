package lwip

/*
#cgo CFLAGS: -I./src/include
#include "lwip/tcp.h"
*/
import "C"
import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"sync"
	"unsafe"

	tun2socks "github.com/eycorsican/go-tun2socks"
)

var (
	tcpAbortError   = errors.New("TCP abort")
	tcpResetError   = errors.New("TCP reset")
	tcpUnknownError = errors.New("unknown TCP error")
)

type tcpConn struct {
	sync.Mutex

	pcb         *C.struct_tcp_pcb
	handler     tun2socks.ConnectionHandler
	network     string
	remoteAddr  string
	remotePort  uint16
	localAddr   string
	localPort   uint16
	connKeyArg  unsafe.Pointer
	connKey     uint32
	closing     bool
	localBuffer *bytes.Buffer
	localLock   sync.RWMutex
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
		remoteAddr:  GetIP4Addr(pcb.local_ip),
		remotePort:  uint16(pcb.local_port),
		localAddr:   GetIP4Addr(pcb.remote_ip),
		localPort:   uint16(pcb.remote_port),
		connKeyArg:  connKeyArg,
		connKey:     connKey,
		closing:     false,
		localBuffer: &bytes.Buffer{},
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
	err := conn.handler.DidReceive(conn, data)
	if err != nil {
		return errors.New(fmt.Sprintf("write proxy failed: %v", err))
	}

	C.tcp_recved(conn.pcb, C.u16_t(len(data)))

	return nil
}

func (conn *tcpConn) writeLocal() error {
	lwipMutex.Lock()
	conn.localLock.Lock()
	defer func() {
		conn.localLock.Unlock()
		lwipMutex.Unlock()
	}()

	var offset = 0
	for {
		pendingSize := conn.localBuffer.Len() - offset
		if pendingSize == 0 {
			conn.localBuffer.Reset()
			break
		} else if pendingSize < 0 {
			log.Fatal("calculating pending data size worng")
		}

		sendSize := int(conn.pcb.snd_buf)
		if pendingSize < sendSize {
			sendSize = pendingSize
		}

		if sendSize == 0 {
			if offset != 0 {
				conn.localBuffer = bytes.NewBuffer(conn.localBuffer.Bytes()[offset:])
			}
			break
		}

		err := C.tcp_write(conn.pcb, unsafe.Pointer(&(conn.localBuffer.Bytes()[offset])), C.u16_t(sendSize), C.TCP_WRITE_FLAG_COPY)
		if err == C.ERR_OK {
			offset += sendSize
			continue
		} else {
			conn.localBuffer = bytes.NewBuffer(conn.localBuffer.Bytes()[offset:])
			break
		}
	}

	err := C.tcp_output(conn.pcb)
	if err != C.ERR_OK {
		log.Fatal("tcp_output error")
	}

	return nil
}

func (conn *tcpConn) Write(data []byte) error {
	conn.localLock.Lock()
	_, err := conn.localBuffer.ReadFrom(bytes.NewReader(data))
	conn.localLock.Unlock()
	if err != nil {
		return errors.New("write local buffer failed")
	}

	go conn.writeLocal()

	return nil
}

func (conn *tcpConn) Sent(len uint16) {
	conn.handler.DidSend(conn, len)
	conn.CheckState()
}

func (conn *tcpConn) isClosing() bool {
	conn.Lock()
	defer conn.Unlock()
	return conn.closing
}

func (conn *tcpConn) localBufferLen() int {
	conn.localLock.RLock()
	defer conn.localLock.RUnlock()
	return conn.localBuffer.Len()
}

func (conn *tcpConn) CheckState() {
	// Still have data to send
	if conn.localBufferLen() > 0 {
		go conn.writeLocal()
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
	conn.Close()
	conn.CheckState()
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
