package lwip

/*
#cgo CFLAGS: -I./src/include
#include "lwip/udp.h"

extern void UDPRecvFn(void *arg, struct udp_pcb *pcb, struct pbuf *p, const ip_addr_t *addr, u16_t port, const ip_addr_t *dest_addr, u16_t dest_port);

void
set_udp_recv_callback(struct udp_pcb *pcb, void *recv_arg) {
	udp_recv(pcb, UDPRecvFn, recv_arg);
}
*/
import "C"
import (
	"unsafe"
)

func SetUDPRecvCallback(pcb *C.struct_udp_pcb, recvArg unsafe.Pointer) {
	C.set_udp_recv_callback(pcb, recvArg)
}
