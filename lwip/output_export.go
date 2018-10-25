package lwip

/*
#cgo CFLAGS: -I./src/include
#include "lwip/tcp.h"
*/
import "C"
import (
	"log"
	"unsafe"
)

//export Output
func Output(p *C.struct_pbuf) C.err_t {
	// In most case, all data are in the same pbuf struct, data copying can be avoid by
	// backing Go slice with C array. Buf if there are multiple pbuf structs holding the
	// data, we must copy data for sending them in one pass.
	if p.tot_len == p.len {
		buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:int(p.len):int(p.len)]
		_, err := OutputFn(buf)
		if err != nil {
			log.Fatal("failed to output packets from stack: %v", err)
		}
	} else {
		buf := NewBytes(int(p.tot_len))
		C.pbuf_copy_partial(p, unsafe.Pointer(&buf[0]), p.tot_len, 0) // data copy here!
		_, err := OutputFn(buf[:int(p.tot_len)])
		FreeBytes(buf)
		if err != nil {
			log.Fatal("failed to output packets from stack: %v", err)
		}
	}

	return C.ERR_OK
}
