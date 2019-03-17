// +build fakedns

package main

import (
	"flag"

	"github.com/eycorsican/go-tun2socks/common/dns/fakedns"
)

func init() {
	args.EnableFakeDns = flag.Bool("fakeDns", false, "Enable fake DNS (SOCKS and Shadowsocks handler)")

	addPostFlagsInitFn(func() {
		if *args.EnableFakeDns {
			fakeDns = fakedns.NewSimpleFakeDns()
		} else {
			fakeDns = nil
		}
	})
}
