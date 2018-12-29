package core

/*
#cgo CFLAGS: -I./src/include
#include "lwip/pbuf.h"
#include "lwip/timeouts.h"
#include "lwip/tcp.h"

err_t
input(struct pbuf *p)
{
	return (*netif_list).input(p, netif_list);
}
*/
import "C"
import (
	"unsafe"
)

func Input(pkt []byte) (int, error) {
	if len(pkt) == 0 {
		return 0, nil
	}

	buf := C.pbuf_alloc_reference(unsafe.Pointer(&pkt[0]), C.u16_t(len(pkt)), C.PBUF_ROM)
	lwipMutex.Lock()
	err := C.input(buf)
	if err != C.ERR_OK {
		C.pbuf_free(buf)
		// TODO
		panic("why failed!?")
	}
	lwipMutex.Unlock()
	return len(pkt), nil
}
