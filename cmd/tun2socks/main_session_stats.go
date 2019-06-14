// +build stats

package main

import (
	"github.com/eycorsican/go-tun2socks/common/stats"
)

func init() {
	addPostFlagsInitFn(func() {
		if *args.Stats {
			sessionStater = stats.NewSimpleSessionStater()
		} else {
			sessionStater = nil
		}
	})
}
