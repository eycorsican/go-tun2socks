package core

/*
#cgo CFLAGS: -I./src/include
#include "lwip/tcp.h"
*/
import "C"
import (
	"context"
	"errors"
	"fmt"
	// "log"
	"math/rand"
	"net"
	"sync"
	"unsafe"
)

type tcpConn struct {
	sync.Mutex

	pcb         *C.struct_tcp_pcb
	handler     ConnectionHandler
	network     string
	remoteAddr  net.Addr
	localAddr   net.Addr
	connKeyArg  unsafe.Pointer
	connKey     uint32
	closing     bool
	localClosed bool
	aborting    bool
	ctx         context.Context
	cancel      context.CancelFunc

	// Data from remote not yet write to local will buffer into this channel.
	localWriteCh    chan []byte
	localWriteSubCh chan []byte
}

// func checkTCPConns() {
// 	tcpConns.Range(func(_, c interface{}) bool {
// 		state := c.(*tcpConn).pcb.state
// 		if c.(*tcpConn).pcb == nil ||
// 			state == C.CLOSED ||
// 			state == C.CLOSE_WAIT {
// 			c.(*tcpConn).Release()
// 		}
// 		return true
// 	})
// }

func NewTCPConnection(pcb *C.struct_tcp_pcb, handler ConnectionHandler) (Connection, error) {
	// prepare key
	connKeyArg := NewConnKeyArg()
	connKey := rand.Uint32()
	SetConnKeyVal(unsafe.Pointer(connKeyArg), connKey)

	if tcpConnectionHandler == nil {
		return nil, errors.New("no registered TCP connection handlers found")
	}

	ctx, cancel := context.WithCancel(context.Background())

	conn := &tcpConn{
		pcb:             pcb,
		handler:         handler,
		network:         "tcp",
		localAddr:       ParseTCPAddr(IPAddrNTOA(pcb.remote_ip), uint16(pcb.remote_port)),
		remoteAddr:      ParseTCPAddr(IPAddrNTOA(pcb.local_ip), uint16(pcb.local_port)),
		connKeyArg:      connKeyArg,
		connKey:         connKey,
		closing:         false,
		localClosed:     false,
		aborting:        false,
		ctx:             ctx,
		cancel:          cancel,
		localWriteCh:    make(chan []byte, 32),
		localWriteSubCh: make(chan []byte, 1),
	}

	// Associate conn with key and save to the global map.
	tcpConns.Store(connKey, conn)

	// go checkTCPConns()

	// Pass the key as arg for subsequent tcp callbacks.
	C.tcp_arg(pcb, unsafe.Pointer(connKeyArg))

	SetTCPRecvCallback(pcb)
	SetTCPSentCallback(pcb)
	SetTCPErrCallback(pcb)
	SetTCPPollCallback(pcb, C.u8_t(1)) // interval 1 means Poll will be called twice a second

	err := handler.Connect(conn, conn.RemoteAddr())
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func (conn *tcpConn) RemoteAddr() net.Addr {
	return conn.remoteAddr
}

func (conn *tcpConn) LocalAddr() net.Addr {
	return conn.localAddr
}

func (conn *tcpConn) Receive(data []byte) error {
	if conn.isClosing() {
		return errors.New(fmt.Sprintf("connection %v->%v was closed by remote", conn.LocalAddr(), conn.RemoteAddr()))
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
				// If writing is not success, buffer to the sub channel, and next time
				// we try to read from the sub channel first. Using a sub channel here
				// because the data must be sent in correct order and we have no way
				// to prepend data to the head of a channel.
				conn.localWriteSubCh <- data
				break Loop
			}
		default:
			break Loop
		}
	}

	// Actually send data.
	C.tcp_output(conn.pcb)
	// err := C.tcp_output(conn.pcb)
	// if err != C.ERR_OK {
	// 	log.Printf("tcp_output error with lwip error code: %v", int(err))
	// }
}

// tcpWrite enqueues data to snd_buf, and treats ERR_MEM returned by tcp_write not an error,
// but instead tells the caller that data is not successfully enqueued, and should try
// again another time. By calling this function, the lwIP thread is assumed to be already
// locked by the caller.
func (conn *tcpConn) tcpWrite(data []byte) (bool, error) {
	if len(data) <= int(conn.pcb.snd_buf) {
		// Enqueue data, data copy here! Copying is required because lwIP must keep the data until they
		// are acknowledged (receiving ACK segments) by other hosts for retransmission purposes, it's
		// not obvious how to implement zero-copy here.
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
	if conn.isLocalClosed() {
		return 0, errors.New(fmt.Sprintf("connection %v->%v was closed by local", conn.LocalAddr(), conn.RemoteAddr()))
	}

	var written = false
	var err error

	// If there isn't any pending data left, we can try to write the data first to avoid one copy,
	// if there is pending data not yet sent, we must copy and buffer the data in order to maintain
	// the transmission order.
	if !conn.hasPendingLocalData() {
		lwipMutex.Lock()
		written, err = conn.tcpWrite(data)
		lwipMutex.Unlock()
		if err != nil {
			return 0, err
		}
	}

	if !written {
		select {
		// Buffer the data here and try sending it later, one could set a smaller localWriteCh size
		// to limit data copying times and memory usage, by sacrificing performance. But writing data
		// to local is quite fast, thus it should be safe even has a size of 1 localWriteCh.
		case conn.localWriteCh <- append([]byte(nil), data...): // data copy here!
		case <-conn.ctx.Done():
			return 0, conn.ctx.Err()
		}
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

func (conn *tcpConn) isLocalClosed() bool {
	conn.Lock()
	defer conn.Unlock()
	return conn.localClosed
}

func (conn *tcpConn) hasPendingLocalData() bool {
	if len(conn.localWriteCh) > 0 || len(conn.localWriteSubCh) > 0 {
		return true
	}
	return false
}

func (conn *tcpConn) CheckState() error {
	// Still have data to send
	if conn.hasPendingLocalData() && !conn.isLocalClosed() {
		go conn.tryWriteLocal()
		// Return and wait for the Sent() callback to be called, and then check again.
		return NewLWIPError(LWIP_ERR_OK)
	}

	if conn.isClosing() || conn.isLocalClosed() {
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

func (conn *tcpConn) setLocalClosed() error {
	conn.Lock()
	defer conn.Unlock()

	conn.localClosed = true
	return nil
}

func (conn *tcpConn) closeInternal() error {
	C.tcp_arg(conn.pcb, nil)
	C.tcp_recv(conn.pcb, nil)
	C.tcp_sent(conn.pcb, nil)
	C.tcp_err(conn.pcb, nil)
	C.tcp_poll(conn.pcb, nil, 0)

	conn.Release()

	conn.cancel()

	// TODO: may return ERR_MEM if no memory to allocate segments use for closing the conn,
	// should check and try again in Sent() for Poll() callbacks.
	err := C.tcp_close(conn.pcb)
	if err == C.ERR_OK {
		return nil
	} else {
		return errors.New(fmt.Sprint("close TCP connection failed, lwip error code %d", int(err)))
	}
}

func (conn *tcpConn) abortInternal() {
	// log.Printf("abort TCP connection %v->%v", conn.LocalAddr(), conn.RemoteAddr())
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
	// log.Printf("error on TCP connection %v->%v: %v", conn.LocalAddr(), conn.RemoteAddr(), err)
	conn.Release()
	conn.cancel()
	conn.handler.DidClose(conn)
}

func (conn *tcpConn) LocalDidClose() error {
	// log.Printf("local close TCP connection %v->%v", conn.LocalAddr(), conn.RemoteAddr())
	conn.handler.LocalDidClose(conn)
	conn.setLocalClosed()    // flag closing
	return conn.CheckState() // check pending data
}

func (conn *tcpConn) Release() {
	if _, found := tcpConns.Load(conn.connKey); found {
		FreeConnKeyArg(conn.connKeyArg)
		tcpConns.Delete(conn.connKey)
	}
	// log.Printf("ended TCP connection %v->%v", conn.LocalAddr(), conn.RemoteAddr())
}

func (conn *tcpConn) Poll() error {
	return conn.CheckState()
}
