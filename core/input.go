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

func Input(pkt []byte) (int, error) {
	if len(pkt) == 0 {
		return 0, nil
	}

	lwipMutex.Lock()
	defer lwipMutex.Unlock()

	// TODO Copy the data only when lwip need to keep it, e.g. in case
	// we are returning ERR_CONN in tcpRecvFn.
	buf := C.pbuf_alloc(C.PBUF_RAW, C.u16_t(len(pkt)), C.PBUF_POOL)
	C.pbuf_take(buf, unsafe.Pointer(&pkt[0]), C.u16_t(len(pkt)))
	err := C.input(buf)
	if err != C.ERR_OK {
		C.pbuf_free(buf)
		return 0, errors.New("packet not handled")
	}
	return len(pkt), nil
}
