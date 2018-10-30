package core

/*
#cgo CFLAGS: -I./src/include
#include "lwip/tcp.h"
#include <stdlib.h>
*/
import "C"
import (
	"fmt"
	// "log"
	"net"
	"sync"
	"unsafe"
)

// ipaddr_ntoa() is using a global static buffer to return result,
// reentrants are not allowed, caller is required to lock lwipMutex.
func IPAddrNTOA(ipaddr C.struct_ip_addr) string {
	return C.GoString(C.ipaddr_ntoa(&ipaddr))
}

func IPAddrATON(cp string, addr *C.struct_ip_addr) {
	ccp := C.CString(cp)
	C.ipaddr_aton(ccp, addr)
	C.free(unsafe.Pointer(ccp))
}

func ParseTCPAddr(addr string, port uint16) net.Addr {
	ip := net.ParseIP(addr).To4()
	if ip != nil {
		// Seems an IPv4 address.
		netAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", addr, port))
		if err != nil {
			// log.Fatalf("resolve TCP address %s:%d failed: %v", addr, port, err)
			return nil
		}
		return netAddr
	}
	ip = net.ParseIP(addr).To16()
	if ip != nil {
		// Seems an IPv6 address.
		netAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("[%s]:%d", addr, port))
		if err != nil {
			// log.Fatalf("resolve TCP address %s:%d failed: %v", addr, port, err)
			return nil
		}
		return netAddr
	}
	return nil
}

func ParseUDPAddr(addr string, port uint16) net.Addr {
	ip := net.ParseIP(addr).To4()
	if ip != nil {
		// Seems an IPv4 address.
		netAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, port))
		if err != nil {
			// log.Fatalf("resolve UDP address %s:%d failed: %v", addr, port, err)
			return nil
		}
		return netAddr
	}
	ip = net.ParseIP(addr).To16()
	if ip != nil {
		// Seems an IPv6 address.
		netAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("[%s]:%d", addr, port))
		if err != nil {
			// log.Fatalf("resolve UDP address %s:%d failed: %v", addr, port, err)
			return nil
		}
		return netAddr
	}
	return nil
}

func GetSyncMapLen(m sync.Map) int {
	length := 0
	m.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	return length
}
