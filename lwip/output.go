package lwip

/*
#cgo CFLAGS: -I./include
#include "lwip/tcp.h"

extern err_t Output(struct pbuf *p);

err_t
output(struct netif *netif, struct pbuf *p, const ip4_addr_t *ipaddr)
{
	return Output(p);
}

void
set_output()
{
	if (netif_list != NULL) {
		(*netif_list).output = output;
	}
}
*/
import "C"
import (
	"errors"
)

func defaultOutputFn(data []byte) (int, error) {
	return 0, errors.New("output function not set")
}

var OutputFn func([]byte) (int, error) = defaultOutputFn

func RegisterOutputFn(fn func([]byte) (int, error)) {
	OutputFn = fn
	C.set_output()
}
