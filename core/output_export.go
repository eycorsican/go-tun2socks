package core

/*
#cgo CFLAGS: -I./c/include
#include "lwip/tcp.h"
*/
import "C"
import (
	"unsafe"
)

//export output
func output(p *C.struct_pbuf) C.err_t {
	// In most case, all data are in the same pbuf struct, data copying can be avoid by
	// backing Go slice with C array. Buf if there are multiple pbuf structs holding the
	// data, we must copy data for sending them in one pass.
	if p.tot_len == p.len {
		buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:int(p.len):int(p.len)]
		OutputFn(buf)
	} else {
		buf := NewBytes(int(p.tot_len))
		C.pbuf_copy_partial(p, unsafe.Pointer(&buf[0]), p.tot_len, 0) // data copy here!
		OutputFn(buf[:int(p.tot_len)])
		FreeBytes(buf)
	}
	return C.ERR_OK
}
