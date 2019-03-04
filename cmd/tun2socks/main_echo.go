// +build echo

package main

import (
	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/proxy/echo"
)

func init() {
	registerHandlerCreater("echo", func() {
		core.RegisterTCPConnectionHandler(echo.NewTCPHandler())
		core.RegisterUDPConnectionHandler(echo.NewUDPHandler())
	})
}
