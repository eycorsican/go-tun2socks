// +build linux darwin

package lwip

/*
#cgo CFLAGS: -I./src/include
#include "lwip/init.h"
*/
import "C"

func lwipInit() {
	C.lwip_init() // Initialze modules.
}
