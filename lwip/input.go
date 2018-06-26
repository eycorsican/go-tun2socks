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
	"errors"
	"log"
	"unsafe"
)

func Input(pkt []byte) error {
	buf := C.pbuf_alloc(C.PBUF_RAW, C.u16_t(len(pkt)), C.PBUF_RAM)
	C.pbuf_take(buf, unsafe.Pointer(&pkt[0]), C.u16_t(len(pkt)))

	// `buf` will be freed by lwip before return.
	lwipMutex.Lock()
	err := C.input(buf)
	C.sys_check_timeouts()
	lwipMutex.Unlock()

	// Currently input() always return ERR_OK, but we check it anyway.
	if err != C.ERR_OK {
		log.Printf("failed to input data: %v", err)
		return errors.New("failed to input packet")
	}
	return nil
}
