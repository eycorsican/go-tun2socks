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
	// TODO: p.tot_len != p.len, have multiple pbuf in the chain?
	if p.tot_len != p.len {
		log.Fatal("p.tot_len != p.len (%v != %v)", p.tot_len, p.len)
	}

	// create Go slice backed by C array, the slice will not garbage collect by Go runtime
	buf := (*[1 << 30]byte)(unsafe.Pointer(p.payload))[:int(p.tot_len):int(p.tot_len)]
	_, err := OutputFn(buf)
	if err != nil {
		log.Printf("failed to output packets from stack: %v", err)
		return C.ERR_OK
	}
	return C.ERR_OK
}
