// +build stats

package main

import (
	"github.com/eycorsican/go-tun2socks/common/stats/session"
)

func init() {
	addPostFlagsInitFn(func() {
		if *args.Stats {
			sessionStater = session.NewSimpleSessionStater()
			sessionStater.Start()
		} else {
			sessionStater = nil
		}
	})
}
