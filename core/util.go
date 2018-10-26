package core

/*
#cgo CFLAGS: -I./src/include
#include "lwip/tcp.h"

char*
get_ip4addr(ip_addr_t ipaddr)
{
	return ip4addr_ntoa(&ipaddr.u_addr.ip4);
}
*/
import "C"
import (
	"fmt"
	"net"
	"sync"
)

func GetIP4Addr(ipaddr C.struct_ip_addr) string {
	return C.GoString(C.get_ip4addr(ipaddr))
}

func MustResolveTCPAddr(addr string, port uint16) net.Addr {
	netAddr, _ := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", addr, port))
	return netAddr
}

func MustResolveUDPAddr(addr string, port uint16) net.Addr {
	netAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", addr, port))
	return netAddr
}

func GetSyncMapLen(m sync.Map) int {
	length := 0
	m.Range(func(_, _ interface{}) bool {
		length++
		return true
	})
	return length
}
