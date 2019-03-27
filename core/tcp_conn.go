package core

/*
#cgo CFLAGS: -I./c/include
#include "lwip/tcp.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"math/rand"
	"net"
	"sync"
	"unsafe"
)

type tcpConnState uint

const (
	tcpNewConn tcpConnState = iota
	tcpConnecting
	tcpConnected
	tcpClosing
	tcpLocalClosed
	tcpAborting
	tcpErrored
)

type tcpConn struct {
	sync.Mutex

	pcb        *C.struct_tcp_pcb
	handler    TCPConnHandler
	remoteAddr net.Addr
	localAddr  net.Addr
	connKeyArg unsafe.Pointer
	connKey    uint32
	canWrite   *sync.Cond // Condition variable to implement TCP backpressure.
	state      tcpConnState
}

func newTCPConn(pcb *C.struct_tcp_pcb, handler TCPConnHandler) (TCPConn, error) {
	connKeyArg := newConnKeyArg()
	connKey := rand.Uint32()
	setConnKeyVal(unsafe.Pointer(connKeyArg), connKey)

	// Pass the key as arg for subsequent tcp callbacks.
	C.tcp_arg(pcb, unsafe.Pointer(connKeyArg))

	// Register callbacks.
	setTCPRecvCallback(pcb)
	setTCPSentCallback(pcb)
	setTCPErrCallback(pcb)
	setTCPPollCallback(pcb, C.u8_t(TCP_POLL_INTERVAL))

	conn := &tcpConn{
		pcb:        pcb,
		handler:    handler,
		localAddr:  ParseTCPAddr(ipAddrNTOA(pcb.remote_ip), uint16(pcb.remote_port)),
		remoteAddr: ParseTCPAddr(ipAddrNTOA(pcb.local_ip), uint16(pcb.local_port)),
		connKeyArg: connKeyArg,
		connKey:    connKey,
		canWrite:   sync.NewCond(&sync.Mutex{}),
		state:      tcpNewConn,
	}

	// Associate conn with key and save to the global map.
	tcpConns.Store(connKey, conn)

	// Connecting remote host could take some time, do it in another goroutine
	// to prevent blocking the lwip thread.
	conn.Lock()
	conn.state = tcpConnecting
	conn.Unlock()
	go func() {
		err := handler.Connect(conn, conn.RemoteAddr())
		if err != nil {
			conn.Abort()
		} else {
			conn.Lock()
			conn.state = tcpConnected
			conn.Unlock()
		}
	}()

	return conn, NewLWIPError(LWIP_ERR_OK)
}

func (conn *tcpConn) RemoteAddr() net.Addr {
	return conn.remoteAddr
}

func (conn *tcpConn) LocalAddr() net.Addr {
	return conn.localAddr
}

func (conn *tcpConn) receiveCheck() error {
	conn.Lock()
	defer conn.Unlock()

	switch conn.state {
	case tcpNewConn:
		fallthrough
	case tcpConnecting:
		return NewLWIPError(LWIP_ERR_CONN)
	case tcpAborting:
		conn.abortInternal()
		return NewLWIPError(LWIP_ERR_ABRT)
	case tcpClosing:
		conn.closeInternal()
		return NewLWIPError(LWIP_ERR_OK)
	case tcpConnected:
		return nil
	default:
		return NewLWIPError(LWIP_ERR_CONN)
	}
	return nil
}

func (conn *tcpConn) Receive(data []byte) error {
	if err := conn.receiveCheck(); err != nil {
		return err
	}
	err := conn.handler.DidReceive(conn, data)
	if err != nil {
		conn.abortInternal()
		conn.canWrite.Broadcast()
		return NewLWIPError(LWIP_ERR_ABRT)
	}
	C.tcp_recved(conn.pcb, C.u16_t(len(data)))
	return NewLWIPError(LWIP_ERR_OK)
}

// writeInternal enqueues data to snd_buf, and treats ERR_MEM returned by tcp_write not an error,
// but instead tells the caller that data is not successfully enqueued, and should try
// again another time. By calling this function, the lwIP thread is assumed to be already
// locked by the caller.
func (conn *tcpConn) writeInternal(data []byte) (int, error) {
	err := C.tcp_write(conn.pcb, unsafe.Pointer(&data[0]), C.u16_t(len(data)), C.TCP_WRITE_FLAG_COPY)
	if err == C.ERR_OK {
		C.tcp_output(conn.pcb)
		return len(data), nil
	} else if err == C.ERR_MEM {
		return 0, nil
	}
	return 0, fmt.Errorf("lwip tcp_write failed with error code: %v", int(err))
}

func (conn *tcpConn) writeCheck() error {
	conn.Lock()
	defer conn.Unlock()

	switch conn.state {
	case tcpErrored:
		return fmt.Errorf("connection %v->%v encountered a fatal error", conn.LocalAddr(), conn.RemoteAddr())
	case tcpAborting:
		return fmt.Errorf("connection %v->%v is aborting", conn.LocalAddr(), conn.RemoteAddr())
	case tcpLocalClosed:
		return fmt.Errorf("connection %v->%v was closed by local", conn.LocalAddr(), conn.RemoteAddr())
	case tcpConnected:
		return nil
	default:
		// It's not likely we will get here.
		return fmt.Errorf("connection %v->%v encountered an unknown error", conn.LocalAddr(), conn.RemoteAddr())
	}
	return nil
}

func (conn *tcpConn) Write(data []byte) (int, error) {
	totalWritten := 0

	conn.canWrite.L.Lock()
	defer conn.canWrite.L.Unlock()

	for len(data) > 0 {
		if err := conn.writeCheck(); err != nil {
			return totalWritten, err
		}

		lwipMutex.Lock()
		toWrite := len(data)
		if toWrite > int(conn.pcb.snd_buf) {
			// Write at most the size of the LWIP buffer.
			toWrite = int(conn.pcb.snd_buf)
		}
		if toWrite > 0 {
			written, err := conn.writeInternal(data[0:toWrite])
			totalWritten += written
			if err != nil {
				lwipMutex.Unlock()
				return totalWritten, err
			}
			data = data[written:len(data)]
		}
		lwipMutex.Unlock()
		if len(data) == 0 {
			break // Don't block if all the data has been written.
		}
		conn.canWrite.Wait()
	}

	return totalWritten, nil
}

func (conn *tcpConn) Sent(len uint16) error {
	// Some packets are acknowledged by local client, check if any pending data to send.
	return conn.CheckState()
}

func (conn *tcpConn) CheckState() error {
	conn.Lock()
	defer conn.Unlock()

	switch conn.state {
	case tcpAborting:
		conn.abortInternal()
		return NewLWIPError(LWIP_ERR_ABRT)
	case tcpClosing:
		fallthrough
	case tcpLocalClosed:
		conn.closeInternal()
		return NewLWIPError(LWIP_ERR_OK)
	}

	// Signal the writer to try writting.
	conn.canWrite.Broadcast()

	return NewLWIPError(LWIP_ERR_OK)
}

func (conn *tcpConn) Close() error {
	lwipMutex.Lock()
	C.tcp_shutdown(conn.pcb, 0, 1) // Close the TX side ASAP.
	lwipMutex.Unlock()

	conn.Lock()
	defer conn.Unlock()

	// Close maybe called outside of lwIP thread, we should not call tcp_close() in this
	// function, instead just make a flag to indicate we are closing the connection.
	conn.state = tcpClosing
	return nil
}

func (conn *tcpConn) setLocalClosed() error {
	conn.Lock()
	defer conn.Unlock()

	conn.state = tcpLocalClosed
	conn.canWrite.Broadcast()
	return nil
}

func (conn *tcpConn) setErrored() error {
	conn.Lock()
	defer conn.Unlock()

	conn.state = tcpErrored
	conn.canWrite.Broadcast()
	return nil
}

func (conn *tcpConn) closeInternal() error {
	C.tcp_arg(conn.pcb, nil)
	C.tcp_recv(conn.pcb, nil)
	C.tcp_sent(conn.pcb, nil)
	C.tcp_err(conn.pcb, nil)
	C.tcp_poll(conn.pcb, nil, 0)

	conn.release()

	// TODO: may return ERR_MEM if no memory to allocate segments use for closing the conn,
	// should check and try again in Sent() for Poll() callbacks.
	err := C.tcp_close(conn.pcb)
	if err == C.ERR_OK {
		return nil
	} else {
		return errors.New(fmt.Sprintf("close TCP connection failed, lwip error code %d", int(err)))
	}
}

func (conn *tcpConn) abortInternal() {
	conn.release()
	C.tcp_abort(conn.pcb)
}

func (conn *tcpConn) Abort() {
	conn.Lock()
	defer conn.Unlock()

	conn.state = tcpAborting
	conn.canWrite.Broadcast()
}

// The corresponding pcb is already freed when this callback is called
func (conn *tcpConn) Err(err error) {
	conn.release()
	conn.handler.DidClose(conn)
	conn.setErrored()
}

func (conn *tcpConn) LocalDidClose() error {
	conn.handler.LocalDidClose(conn)
	conn.setLocalClosed()
	return conn.CheckState()
}

func (conn *tcpConn) release() {
	if _, found := tcpConns.Load(conn.connKey); found {
		freeConnKeyArg(conn.connKeyArg)
		tcpConns.Delete(conn.connKey)
	}
}

func (conn *tcpConn) Poll() error {
	return conn.CheckState()
}
