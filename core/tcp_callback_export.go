package core

/*
#cgo CFLAGS: -I./src/include
#include "lwip/tcp.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

// These exported callback functions must be placed in a seperated file.
//
// See also:
// https://github.com/golang/go/issues/20639
// https://golang.org/cmd/cgo/#hdr-C_references_to_Go

//export tcpAcceptFn
func tcpAcceptFn(arg unsafe.Pointer, newpcb *C.struct_tcp_pcb, err C.err_t) C.err_t {
	if err != C.ERR_OK {
		return err
	}

	if tcpConnectionHandler == nil {
		panic("must register a TCP connection handler")
	}

	if _, err2 := newTCPConnection(newpcb, tcpConnectionHandler); err2 != nil {
		if err2.(*lwipError).Code == LWIP_ERR_ABRT {
			return C.ERR_ABRT
		} else if err2.(*lwipError).Code == LWIP_ERR_OK {
			return C.ERR_OK
		} else {
			return C.ERR_CONN
		}
	}

	return C.ERR_OK
}

//export tcpRecvFn
func tcpRecvFn(arg unsafe.Pointer, tpcb *C.struct_tcp_pcb, p *C.struct_pbuf, err C.err_t) C.err_t {
	if err != C.ERR_OK && err != C.ERR_ABRT {
		return err
	}

	// Only free the pbuf when returning ERR_OK or ERR_ABRT.
	defer func() {
		if p != nil {
			C.pbuf_free(p)
		}
	}()

	conn, ok := tcpConns.Load(getConnKeyVal(arg))
	if !ok {
		// The connection does not exists.
		C.tcp_abort(tpcb)
		return C.ERR_ABRT
	}

	if p == nil {
		// The connection has been closed.
		err := conn.(Connection).LocalDidClose()
		if err.(*lwipError).Code == LWIP_ERR_ABRT {
			return C.ERR_ABRT
		} else if err.(*lwipError).Code == LWIP_ERR_OK {
			return C.ERR_OK
		}
	}

	// create Go slice backed by C array, the slice will not garbage collect by Go runtime
	buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:int(p.tot_len):int(p.tot_len)]
	recvErr := conn.(Connection).Receive(buf)
	if recvErr != nil {
		if recvErr.(*lwipError).Code == LWIP_ERR_ABRT {
			return C.ERR_ABRT
		} else if recvErr.(*lwipError).Code == LWIP_ERR_OK {
			return C.ERR_OK
		}
	}

	return C.ERR_OK
}

//export tcpSentFn
func tcpSentFn(arg unsafe.Pointer, tpcb *C.struct_tcp_pcb, len C.u16_t) C.err_t {
	if conn, ok := tcpConns.Load(getConnKeyVal(arg)); ok {
		err := conn.(Connection).Sent(uint16(len))
		if err.(*lwipError).Code == LWIP_ERR_ABRT {
			return C.ERR_ABRT
		} else {
			return C.ERR_OK
		}
	} else {
		C.tcp_abort(tpcb)
		return C.ERR_ABRT
	}
}

//export tcpErrFn
func tcpErrFn(arg unsafe.Pointer, err C.err_t) {
	if conn, ok := tcpConns.Load(getConnKeyVal(arg)); ok {
		switch err {
		case C.ERR_ABRT:
			// Aborted through tcp_abort or by a TCP timer
			conn.(Connection).Err(errors.New("connection aborted"))
		case C.ERR_RST:
			// The connection was reset by the remote host
			conn.(Connection).Err(errors.New("connection reseted"))
		default:
			conn.(Connection).Err(errors.New(fmt.Sprintf("lwip error code %v", int(err))))
		}
	}
}

//export tcpPollFn
func tcpPollFn(arg unsafe.Pointer, tpcb *C.struct_tcp_pcb) C.err_t {
	if conn, ok := tcpConns.Load(getConnKeyVal(arg)); ok {
		err := conn.(Connection).Poll()
		if err.(*lwipError).Code == LWIP_ERR_ABRT {
			return C.ERR_ABRT
		} else if err.(*lwipError).Code == LWIP_ERR_OK {
			return C.ERR_OK
		}
	} else {
		C.tcp_abort(tpcb)
		return C.ERR_ABRT
	}
	return C.ERR_OK
}
