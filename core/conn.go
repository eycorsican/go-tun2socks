package core

import (
	"net"
	"time"
)

// TCPConn abstracts a TCP connection comming from TUN. This connection
// should be handled by a registered TCP proxy handler.
type TCPConn interface {
	// RemoteAddr returns the destination network address.
	RemoteAddr() net.Addr

	// LocalAddr returns the local client network address.
	LocalAddr() net.Addr

	// Receive receives data from TUN.
	Receive(data []byte) error

	// Read reads data comming from TUN, note that it reads from an
	// underlying pipe that the writer writes in the lwip thread,
	// write op blocks until previous written data is consumed, one
	// should read out all data as soon as possible.
	Read(data []byte) (int, error)

	// Write writes data to TUN.
	Write(data []byte) (int, error)

	// Sent will be called when sent data has been acknowledged by clients.
	Sent(len uint16) error

	// Close closes the connection.
	Close() error

	// Abort aborts the connection to client by sending a RST segment.
	Abort()

	// Err will be called when a fatal error has occurred on the connection.
	Err(err error)

	// LocalDidClose will be called when local client has close the connection.
	LocalDidClose() error

	// Poll will be periodically called by timers.
	Poll() error

	SetDeadline(t time.Time) error
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
}

// TCPConn abstracts a UDP connection comming from TUN. This connection
// should be handled by a registered UDP proxy handler.
type UDPConn interface {
	// LocalAddr returns the local client network address.
	LocalAddr() net.Addr

	// ReceiveTo receives data from TUN, and the received data should be sent to addr.
	ReceiveTo(data []byte, addr net.Addr) error

	// WriteFrom writes data to TUN, which was received from addr. addr will be set as
	// source address of IP packets that output to TUN.
	WriteFrom(data []byte, addr net.Addr) (int, error)

	// Close closes the connection.
	Close() error
}
