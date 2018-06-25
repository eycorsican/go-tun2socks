package tun2socks

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
	Write(data []byte) error

	// Sent will be called when sent data has been acknowledged by clients.
	Sent(len uint16)

	// Close closes the connection.
	Close() error

	// Abort aborts the connection to client by sending a RST segment.
	Abort()

	// Err will be called when a fatal error has occurred on the connection.
	Err(err error)

	// Reset resets the connection.
	Reset()

	// LocalDidClose will be called when local client has close the connection.
	LocalDidClose()
}
