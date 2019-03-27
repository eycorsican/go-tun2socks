# go-tun2socks

[![Build Status](https://travis-ci.com/eycorsican/go-tun2socks.svg?branch=master)](https://travis-ci.com/eycorsican/go-tun2socks)

A tun2socks implementation written in Go.

Tested and worked on macOS, Linux and Windows, iOS and Android are also supported (as a library).

## Overview

```
                                                                          core.NewLWIPStack()
                                                   Log process names               +
                                                   Delay ICMP packets              |
                                               Dynamically change routes           |
                                                            |                      |
                                                            |                      |                                    core.RegisterTCPConnHandler()
                                                            |                      |           core.TCPConn
                                                            |     core.Input()     |           core.UDPConn             core.RegisterUDPConnHandler()
                                                            +                      +
                                 +-----------> TUN +----> Filter +-----------> lwIP stack +-----------------------> core.TCPConnHandler/core.UDPConnHandler
                                 |                               <-----------+                                                        +
                                 |                            core.RegisterOutputFn()                                                 |
                                 |                                                                                                    |
Application +-> Routing table +-->                                                                                                    |
                      ^          |                                                                                                    |
                      |          |                                                                                                    |
                      |          |                                                 +------> Destination                               |
                      |          +-----------> Original gateway +---> Internet ----+                                                  |
                      |                                                            +------> Proxy server +--> Destination             |
                      |                                                                                                               |
                      |                                                                                                               |
                      +---------------------------------------------------------------------------------------------------------------+

```

## Main Features

- Support both TCP and UDP
- Support both IPv4 and IPv6
- Support proxy handlers: `SOCKS5`, `Shadowsocks`, `V2Ray`
- ICMP echoing
- DNS fallback
- DNS caching
- Fake DNS

## Usage

```
Usage of tun2socks:
  -applog
    	Enable app logging (V2Ray, Shadowsocks and SOCKS5 handler)
  -delayICMP int
    	Delay ICMP packets for a short period of time, in milliseconds (default 10)
  -disableDNSCache
    	Disable DNS cache (SOCKS5 and Shadowsocks handler)
  -dnsFallback
    	Enable DNS fallback over TCP (overrides the UDP proxy handler).
  -fakeDns
    	Enable fake DNS (SOCKS and Shadowsocks handler)
  -gateway string
    	The gateway adrress of your default network, set this to enable dynamic routing, and root/admin privileges may also required for using dynamic routing (V2Ray only)
  -loglevel string
    	Logging level. (debug, info, warn, error, none) (default "info")
  -proxyCipher string
    	Cipher used for Shadowsocks proxy, available ciphers: AEAD_AES_128_GCM AEAD_AES_192_GCM AEAD_AES_256_GCM AEAD_CHACHA20_POLY1305 AES-128-CFB AES-128-CTR AES-192-CFB AES-192-CTR AES-256-CFB AES-256-CTR CHACHA20-IETF XCHACHA20 (default "AEAD_CHACHA20_POLY1305")
  -proxyPassword string
    	Password used for Shadowsocks proxy
  -proxyServer string
    	Proxy server address (host:port) for socks and Shadowsocks proxies (default "1.2.3.4:1087")
  -proxyType string
    	Proxy handler type, e.g. socks, shadowsocks, v2ray (default "socks")
  -sniffingType string
    	Enable domain sniffing for specific kind of traffic in v2ray (default "http,tls")
  -tunAddr string
    	TUN interface address (default "240.0.0.2")
  -tunDns string
    	DNS resolvers for TUN interface (only need on Windows) (default "114.114.114.114,223.5.5.5")
  -tunGw string
    	TUN interface gateway (default "240.0.0.1")
  -tunMask string
    	TUN interface netmask, as for IPv6, it's the prefixlen (default "255.255.255.0")
  -tunName string
    	TUN interface name (default "tun1")
  -udpTimeout duration
    	Set timeout for UDP proxy connections in SOCKS and Shadowsocks (default 1m0s)
  -vconfig string
    	Config file for v2ray, in JSON format, and note that routing in v2ray could not violate routes in the routing table (default "config.json")
  -version
    	Print version
```

## Build

`go-tun2socks` is using `cgo`, thus a C compiler is required.

```sh
go get github.com/eycorsican/go-tun2socks
cd $GOPATH/src/github.com/eycorsican/go-tun2socks
go get -d ./...
make clean && make build
./build/tun2socks -h
```
### Cross Compiling

An alternative way to build (or cross compile) tun2socks is to use [`xgo`](https://github.com/karalabe/xgo), to use `xgo`, you also need `docker`:

```sh
# install docker: https://docs.docker.com/install

# install xgo
go get github.com/karalabe/xgo

go get github.com/eycorsican/go-tun2socks
cd $GOPATH/src/github.com/eycorsican/go-tun2socks
go get -d ./...
make clean && make release
ls ./build
```

### Customizing Build

The default build behavior is to include all available modules, ends up a fat binary that will contain modules you may not need. It's easy to customize the build to include only modules you want by modifying the `Makefile`, for example, you may build `go-tun2socks` with only the `socks` proxy handler by setting the `BUILD_TAGS` variable before calling `make`:
```
# socks handler only
BUILD_TAGS=socks make

# socks handler with DNS cache
BUILD_TAGS="socks dnscache" make
```

## Run

```sh
./build/tun2socks -tunName tun1 -tunAddr 240.0.0.2 -tunGw 240.0.0.1 -proxyType socks -proxyServer 1.2.3.4:1086
```

Note that the TUN device may have a different name, and it should be a different name on Windows unless you have renamed it, so make sure use `ifconfig`, `ipconfig` or `ip addr` to check it out.

## Create TUN device and Configure Routing Table

Suppose your original gateway is 192.168.0.1. The proxy server address is 1.2.3.4.

The following commands will need root permissions.

### macOS

The program will automatically create a TUN device for you on macOS. To show the created TUN device, use ifconfig.

Delete original gateway:

```sh
route delete default
```

Add our TUN interface as the default gateway:

```sh
route add default 240.0.0.1
```

Add a route for your proxy server to bypass the TUN interface:

```sh
route add 1.2.3.4/32 192.168.0.1
```

### Linux

The program will not create the TUN device for you on Linux. You need to create the TUN device by yourself:

```sh
ip tuntap add mode tun dev tun1
ip addr add 240.0.0.1 dev tun1
ip link set dev tun1 up
```

Delete original gateway:

```sh
ip route del default
```

Add our TUN interface as the default gateway:

```sh
ip route add default via 240.0.0.1
```

Add a route for your proxy server to bypass the TUN interface:

```sh
ip route add 1.2.3.4/32 via 192.168.0.1
```

### Windows

To create a TUN device on Windows, you need [Tap-windows](http://build.openvpn.net/downloads/releases/), refer [here](https://code.google.com/archive/p/badvpn/wikis/tun2socks.wiki) for more information.

Add our TUN interface as the default gateway:

```sh
# Using 240.0.0.1 is not allowed on Windows, we use 10.0.0.1 instead
route add 0.0.0.0 mask 0.0.0.0 10.0.0.1 metric 6
```

Add a route for your proxy server to bypass the TUN interface:

```sh
route add 1.2.3.4 192.168.0.1 metric 5
```

## A few notes for using V2Ray proxy handler
- Using V2Ray proxy handler: `tun2socks -proxyType v2ray -vconfig config.json`
- V2Ray proxy handler dials connections with a [V2Ray Instance](https://github.com/v2ray/v2ray-core/blob/master/functions.go)
- Configuration file must in JSON format
- Proxy server addresses in the configuration file should be IPs and not domains except your system DNS will match "direct" rules
- Configuration file should not contain direct `domain` rules, since they cause infinitely looping requests
- Dynamic routing happens prior to packets input to lwIP, the [V2Ray Router](https://github.com/v2ray/v2ray-core/blob/master/features/routing/router.go) is used to check if the IP packet matching "direct" tag, information available for the matching process are (protocol, destination ip, destination port)
- To enable dynamic routing, just set the `-gateway` argument, for example: `tun2socks -proxyType v2ray -vconfig config.json -gateway 192.168.0.1`
- The tag "direct" is hard coded to identify direct rules, which if dynamic routing is enabled, will indicate adding routes to the original gateway for the corresponding IP packets
- Inbounds are not necessary

## TODO
- Built-in routing rules and routing table management
- Support ICMP packets forwarding
- Make conn handlers [io.ReadWriteCloser](https://golang.org/pkg/io/#ReadWriteCloser)
- Add Close() method for core.LWIPStack
- Support TAP device in order to support IPv6 on Windows

## Development

The core part of this project is the `core` package, it focuses on `tun2socks`'s `2` part, the core package has fully IPv4/IPv6, TCP/UDP support, and only depends on lwIP (including a few [platform-dependent code](https://github.com/eycorsican/go-tun2socks/tree/master/core/src/custom)) and Go's standard library. On the one hand, IP packets input to or output from the `lwIP Stack` that initialized by `core.NewLWIPStack()`, on the other hand, TCP/UDP connections would "socksified" by the core package and can be handled by your own `core.TCPConnHandler`/`core.UDPConnHandler` implementation.

As for the `tun` part, different OS may has it's own interfaces.

For example:
- macOS
  - macOS has TUN/TAP support by its BSD kernel
  - Apple also provides an easy way to filter inbound or outbound IP packets by [`IP Filters`](https://developer.apple.com/library/archive/documentation/Darwin/Conceptual/NKEConceptual/ip_filter_nke/ip_filter_nke.html) (Proxifier seems use this method)
- Linux
  - Linux has TUN/TAP support by the kernel
- Windows
  - Windows has no kernel support for TUN/TAP, but OpenVPN has [implemented one](https://github.com/OpenVPN/tap-windows6)
- iOS
  - Apple provides [`NEPacketTunnelProvider`](https://developer.apple.com/documentation/networkextension/nepackettunnelprovider), and one may read/write IP packets from/to the [`packetFlow`](https://developer.apple.com/documentation/networkextension/nepackettunnelprovider/1406185-packetflow)
- Android
  - I am not familiar with Android, but it uses Linux as kernel so should also has TUN/TAP drivers support
  - Android also provides an easy way to read/write IP packets with [VpnService.Builder](https://developer.android.com/reference/android/net/VpnService.Builder#establish())

Sample code for creating a `lwIP Stack` and doing IP packets inputing/outputing, please refer `cmd/tun2socks/main.go`. Sample code for implementing `core.TCPConnHandler`/`core.UDPConnHandler`, please refer `proxy/*`.

## Creating a Framework for iOS

https://github.com/eycorsican/go-tun2socks-ios

## Creating an AAR library for Android

https://github.com/eycorsican/go-tun2socks-android

## This project is using lwIP 

This project is using a modified version of lwIP, you can checkout this repo to find out what are the changes: https://github.com/eycorsican/lwip

## Many thanks to the following projects
- https://savannah.nongnu.org/projects/lwip
- https://github.com/ambrop72/badvpn
- https://github.com/zhuhaow/tun2socks
- https://github.com/yinghuocho/gotun2socks
- https://github.com/v2ray/v2ray-core
- https://github.com/shadowsocks/go-shadowsocks2
- https://github.com/songgao/water
- https://github.com/nadoo/glider
