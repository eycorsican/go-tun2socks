package lwip

/*
#cgo CFLAGS: -I./include
#include "lwip/tcp.h"
*/
import "C"
import (
	"log"
	"unsafe"
)

//export Output
func Output(p *C.struct_pbuf) C.err_t {
	buf := NewBytes()
	C.pbuf_copy_partial(p, unsafe.Pointer(&buf[0]), p.tot_len, 0)
	_, err := OutputFn(buf[:int(p.tot_len)])
	FreeBytes(buf)
	if err != nil {
		log.Printf("failed to output packets from stack: %v", err)
		return C.ERR_OK
	}
	return C.ERR_OK
}
