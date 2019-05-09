package socks

import (
	"io"
	"net"
	"time"
)

type readerConn struct {
	conn   net.Conn
	reader io.Reader
}

func (c *readerConn) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}
func (c *readerConn) Write(b []byte) (int, error) {
	return c.conn.Write(b)
}
func (c *readerConn) Close() error {
	return c.conn.Close()
}
func (c *readerConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}
func (c *readerConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}
func (c *readerConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}
func (c *readerConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}
func (c *readerConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
