package lwip

import (
	tun2socks "github.com/v2dev/go-tun2socks"
)

type Listener interface {
	Accept(conn tun2socks.Connection)
}

type defaultListener struct{}

var listener Listener = new(defaultListener)

func SetListener(l Listener) {
	listener = l
}

func (l *defaultListener) Accept(conn tun2socks.Connection) {
}
