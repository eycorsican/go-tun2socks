package core

/*
#cgo CFLAGS: -I./c/custom
#include "c/custom/sys_arch.c"
*/
import (
	"C"
	_ "github.com/eycorsican/go-tun2socks/core/c"
)
