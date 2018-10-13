package lwip

/*
#cgo CFLAGS: -I./src/include
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

func Input(pkt []byte) error {
	buf := C.pbuf_alloc(C.PBUF_RAW, C.u16_t(len(pkt)), C.PBUF_RAM)
	C.pbuf_take(buf, unsafe.Pointer(&pkt[0]), C.u16_t(len(pkt)))

	// buf will be freed by lwip.
	lwipMutex.Lock()
	C.input(buf)
	C.sys_check_timeouts()
	lwipMutex.Unlock()

	return nil
}
