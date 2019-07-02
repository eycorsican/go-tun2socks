// +build fakedns

package main

import (
	"flag"

	"github.com/eycorsican/go-tun2socks/common/dns/fakedns"
)

func init() {
	args.EnableFakeDns = flag.Bool("fakeDns", false, "Enable fake DNS")
	args.FakeDnsAddr = flag.String("fakeDnsAddr", ":53", "listen address of fake DNS")
	args.FakeIPRange = flag.String("fakeIPRange", "198.18.0.1/16", "fake IP CIDR range for DNS")

	addPostFlagsInitFn(func() {
		if *args.EnableFakeDns {
			fakeDnsServer, err := fakedns.NewServer(*args.FakeDnsAddr, *args.FakeIPRange)
			if err != nil {
				panic("create fake dns server error")
			}
			fakeDnsServer.StartServer()
			fakeDns = fakeDnsServer
		} else {
			fakeDns = nil
		}
	})
}
