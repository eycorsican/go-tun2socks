// +build stats

package main

import (
	"github.com/eycorsican/go-tun2socks/common/stats/session"
)

func init() {
	args.StatsAddr = flag.String("statsAddr", "localhost:6001", "listen address of stats, open in your browser to view statistics")

	addPostFlagsInitFn(func() {
		if *args.Stats {
			session.StatsAddr = *args.StatsAddr
			sessionStater = session.NewSimpleSessionStater()
			sessionStater.Start()
		} else {
			sessionStater = nil
		}
	})
}
