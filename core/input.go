package core

/*
#cgo CFLAGS: -I./c/include
#include "lwip/pbuf.h"
#include "lwip/tcp.h"

err_t
input(struct pbuf *p)
{
	return (*netif_list).input(p, netif_list);
}
*/
import "C"
import (
	"errors"
	"unsafe"
)

type ipver byte

const (
	ipv4 = 4
	ipv6 = 6
)

type proto byte

const (
	proto_tcp = 6
	proto_udp = 17
)

func peekIPVer(p []byte) (ipver, error) {
	if len(p) < 1 {
		return 0, errors.New("short IP packet")
	}
	return ipver((p[0] & 0xf0) >> 4), nil
}

func peekNextProto(p []byte) (proto, error) {
	ipv, err := peekIPVer(p)
	if err != nil {
		return 0, err
	}

	switch ipv {
	case ipv4:
		if len(p) < 9 {
			return 0, errors.New("short IPv4 packet")
		}
		return proto(p[9]), nil
	case ipv6:
		if len(p) < 6 {
			return 0, errors.New("short IPv6 packet")
		}
		return proto(p[6]), nil
	default:
		return 0, errors.New("unknown IP version")
	}
}

func Input(pkt []byte) (int, error) {
	if len(pkt) == 0 {
		return 0, nil
	}

	nextProto, err := peekNextProto(pkt)
	if err != nil {
		return 0, err
	}

	lwipMutex.Lock()
	defer lwipMutex.Unlock()

	var buf *C.struct_pbuf
	switch nextProto {
	case proto_udp:
		// Copying data is not necessary for UDP, and we would like to
		// have all data in one pbuf.
		buf = C.pbuf_alloc_reference(unsafe.Pointer(&pkt[0]), C.u16_t(len(pkt)), C.PBUF_ROM)
	default:
		// TODO Copy the data only when lwip need to keep it, e.g. in
		// case we are returning ERR_CONN in tcpRecvFn.
		//
		// Allocating from PBUF_POOL results in a pbuf chain that may
		// contain multiple pbufs.
		buf = C.pbuf_alloc(C.PBUF_RAW, C.u16_t(len(pkt)), C.PBUF_POOL)
		C.pbuf_take(buf, unsafe.Pointer(&pkt[0]), C.u16_t(len(pkt)))
	}

	ierr := C.input(buf)
	if ierr != C.ERR_OK {
		C.pbuf_free(buf)
		return 0, errors.New("packet not handled")
	}
	return len(pkt), nil
}
