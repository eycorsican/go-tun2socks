// +build fakedns

package main

import (
	"flag"

	"github.com/eycorsican/go-tun2socks/common/dns/fakedns"
)

func init() {
	args.EnableFakeDns = flag.Bool("fakeDns", false, "Enable fake DNS")
	args.FakeDnsMinIP = flag.String("fakeDnsMinIP", "172.255.0.0", "Minimum fake IP used by fake DNS")
	args.FakeDnsMaxIP = flag.String("fakeDnsMaxIP", "172.255.255.255", "Maximum fake IP used by fake DNS")

	addPostFlagsInitFn(func() {
		if *args.EnableFakeDns {
			fakeDns = fakedns.NewSimpleFakeDns(*args.FakeDnsMinIP, *args.FakeDnsMaxIP)
		} else {
			fakeDns = nil
		}
	})
}
