package core

import (
	"net"
)

// Connection abstracts a TCP/UDP connection comming from TUN. This connection
// should be handled by a registered TCP/UDP proxy handler.
type Connection interface {
	// RemoteAddr returns the destination network address.
	RemoteAddr() net.Addr

	// LocalAddr returns the local client network address.
	LocalAddr() net.Addr

	// Receive receives data from TUN.
	Receive(data []byte) error

	// Write writes data to TUN.
	Write(data []byte) (int, error)

	// Sent will be called when sent data has been acknowledged by clients (TCP only).
	Sent(len uint16) error

	// Close closes the connection (TCP only).
	Close() error

	// Abort aborts the connection to client by sending a RST segment (TCP only).
	Abort()

	// Err will be called when a fatal error has occurred on the connection (TCP only).
	Err(err error)

	// LocalDidClose will be called when local client has close the connection (TCP only).
	LocalDidClose() error

	// Poll will be periodically called by timers (TCP only).
	Poll() error
}
