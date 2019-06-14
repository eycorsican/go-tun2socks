package main

import (
	"github.com/eycorsican/go-tun2socks/common/stats"
)

func init() {
	addPostFlagsInitFn(func() {
		sessionStater = stats.NewSimpleSessionStater()
	})
}
