package core

/*
#cgo CFLAGS: -I./src/include
#include "lwip/tcp.h"
*/
import "C"
import (
	"fmt"
	"log"
	"net"
	"sync"
)

func GetIPAddr(ipaddr C.struct_ip_addr) string {
	return C.GoString(C.ipaddr_ntoa(&ipaddr))
}

func MustResolveTCPAddr(addr string, port uint16) net.Addr {
	ip := net.ParseIP(addr).To4()
	if ip != nil {
		// Seems an IPv4 address.
		netAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", addr, port))
		if err != nil {
			// TODO: Encountering addresses failed to resolve, and they are even not valid
			// IP addresses, no idea why they come in, better handling is needed.
			log.Fatalf("resolve address %s:%d failed: %v", addr, port, err)
		}
		return netAddr
	} else {
		// Seems an IPv6 address.
		netAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("[%s]:%d", addr, port))
		if err != nil {
			log.Fatalf("resolve address %s:%d failed: %v", addr, port, err)
		}
		return netAddr
	}
}

func MustResolveUDPAddr(addr string, port uint16) net.Addr {
	ip := net.ParseIP(addr).To4()
	if ip != nil {
		// Seems an IPv4 address.
		netAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, port))
		if err != nil {
			log.Fatalf("resolve address %s:%d failed: %v", addr, port, err)
		}
		return netAddr
	} else {
		// Seems an IPv6 address.
		netAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("[%s]:%d", addr, port))
		if err != nil {
			log.Fatalf("resolve address %s:%d failed: %v", addr, port, err)
		}
		return netAddr
	}
}

func GetSyncMapLen(m sync.Map) int {
	length := 0
	m.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	return length
}
