package direct

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	"github.com/eycorsican/go-tun2socks/common/log"
	"github.com/eycorsican/go-tun2socks/core"
)

type tcpHandler struct{}

func NewTCPHandler() core.TCPConnHandler {
	return &tcpHandler{}
}

func (h *tcpHandler) handleInput(conn net.Conn, input io.ReadCloser) {
	defer func() {
		conn.Close()
		input.Close()
	}()
	io.Copy(conn, input)
}

func (h *tcpHandler) handleOutput(conn net.Conn, output io.WriteCloser) {
	defer func() {
		conn.Close()
		output.Close()
	}()
	io.Copy(output, conn)
}

func (h *tcpHandler) Handle(conn net.Conn, target net.Addr) error {
	c, err := net.Dial("tcp", target.String())
	if err != nil {
		return err
	}
	go h.handleInput(conn, c)
	go h.handleOutput(conn, c)
	log.Infof("new proxy connection for target: %s:%s", target.Network(), target.String())
	return nil
}
