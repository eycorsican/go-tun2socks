package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/eycorsican/go-tun2socks/common/dns"
	"github.com/eycorsican/go-tun2socks/common/log"
	_ "github.com/eycorsican/go-tun2socks/common/log/simple" // Register a simple logger.
	"github.com/eycorsican/go-tun2socks/core"
	"github.com/eycorsican/go-tun2socks/filter"
	"github.com/eycorsican/go-tun2socks/tun"
)

var version = "undefined"

var handlerCreater = make(map[string]func(), 0)

func registerHandlerCreater(name string, creater func()) {
	handlerCreater[name] = creater
}

var postFlagsInitFn = make([]func(), 0)

func addPostFlagsInitFn(fn func()) {
	postFlagsInitFn = append(postFlagsInitFn, fn)
}

type CmdArgs struct {
	Version         *bool
	TunName         *string
	TunAddr         *string
	TunGw           *string
	TunMask         *string
	TunDns          *string
	ProxyType       *string
	VConfig         *string
	Gateway         *string
	SniffingType    *string
	ProxyServer     *string
	ProxyHost       *string
	ProxyPort       *uint16
	ProxyCipher     *string
	ProxyPassword   *string
	DelayICMP       *int
	UdpTimeout      *time.Duration
	Applog          *bool
	DisableDnsCache *bool
	DnsFallback     *bool
	LogLevel        *string
	EnableFakeDns   *bool
}

type cmdFlag uint

const (
	fProxyServer cmdFlag = iota
	fUdpTimeout
	fApplog
)

var flagCreaters = map[cmdFlag]func(){
	fProxyServer: func() {
		if args.ProxyServer == nil {
			args.ProxyServer = flag.String("proxyServer", "1.2.3.4:1087", "Proxy server address (host:port) for socks and Shadowsocks proxies")
		}
	},
	fUdpTimeout: func() {
		if args.UdpTimeout == nil {
			args.UdpTimeout = flag.Duration("udpTimeout", 1*time.Minute, "Set timeout for UDP proxy connections in SOCKS and Shadowsocks")
		}
	},
	fApplog: func() {
		if args.Applog == nil {
			args.Applog = flag.Bool("applog", false, "Enable app logging (V2Ray, Shadowsocks and SOCKS5 handler)")
		}
	},
}

func (a *CmdArgs) addFlag(f cmdFlag) {
	if fn, found := flagCreaters[f]; found && fn != nil {
		fn()
	} else {
		log.Fatalf("unsupported flag")
	}
}

var args = new(CmdArgs)

var lwipWriter io.Writer

var dnsCache dns.DnsCache

var fakeDns dns.FakeDns

const (
	MTU = 1500
)

func main() {
	args.Version = flag.Bool("version", false, "Print version")
	args.TunName = flag.String("tunName", "tun1", "TUN interface name")
	args.TunAddr = flag.String("tunAddr", "240.0.0.2", "TUN interface address")
	args.TunGw = flag.String("tunGw", "240.0.0.1", "TUN interface gateway")
	args.TunMask = flag.String("tunMask", "255.255.255.0", "TUN interface netmask, as for IPv6, it's the prefixlen")
	args.TunDns = flag.String("tunDns", "114.114.114.114,223.5.5.5", "DNS resolvers for TUN interface (only need on Windows)")
	args.ProxyType = flag.String("proxyType", "socks", "Proxy handler type, e.g. socks, shadowsocks, v2ray")
	args.DelayICMP = flag.Int("delayICMP", 10, "Delay ICMP packets for a short period of time, in milliseconds")
	args.LogLevel = flag.String("loglevel", "info", "Logging level. (debug, info, warn, error, none)")

	flag.Parse()

	if *args.Version {
		fmt.Println(version)
		os.Exit(0)
	}

	// Initialization ops after parsing flags.
	for _, fn := range postFlagsInitFn {
		if fn != nil {
			fn()
		}
	}

	// Set log level.
	switch strings.ToLower(*args.LogLevel) {
	case "debug":
		log.SetLevel(log.DEBUG)
	case "info":
		log.SetLevel(log.INFO)
	case "warn":
		log.SetLevel(log.WARN)
	case "error":
		log.SetLevel(log.ERROR)
	case "none":
		log.SetLevel(log.NONE)
	default:
		panic("unsupport logging level")
	}

	// Open the tun device.
	dnsServers := strings.Split(*args.TunDns, ",")
	tunDev, err := tun.OpenTunDevice(*args.TunName, *args.TunAddr, *args.TunGw, *args.TunMask, dnsServers)
	if err != nil {
		log.Fatalf("failed to open tun device: %v", err)
	}

	// Setup TCP/IP stack.
	lwipWriter := core.NewLWIPStack().(io.Writer)

	// Wrap a writer to delay ICMP packets if delay time is not zero.
	if *args.DelayICMP > 0 {
		log.Infof("ICMP packets will be delayed for %dms", *args.DelayICMP)
		lwipWriter = filter.NewICMPFilter(lwipWriter, *args.DelayICMP).(io.Writer)
	}

	// Wrap a writer to print out processes the creating network connections.
	if args.Applog != nil && *args.Applog {
		log.Infof("App logging is enabled")
		lwipWriter = filter.NewApplogFilter(lwipWriter).(io.Writer)
	}

	// Register TCP and UDP handlers to handle accepted connections.
	if creater, found := handlerCreater[*args.ProxyType]; found {
		creater()
	} else {
		log.Fatalf("unsupported proxy type")
	}

	if args.DnsFallback != nil && *args.DnsFallback {
		// Override the UDP handler with a DNS-over-TCP (fallback) UDP handler.
		if creater, found := handlerCreater["dnsfallback"]; found {
			creater()
		} else {
			log.Fatalf("DNS fallback connection handler not found, build with `dnsfallback` tag")
		}
	}

	// Register an output callback to write packets output from lwip stack to tun
	// device, output function should be set before input any packets.
	core.RegisterOutputFn(func(data []byte) (int, error) {
		return tunDev.Write(data)
	})

	// Copy packets from tun device to lwip stack, it's the main loop.
	go func() {
		_, err := io.CopyBuffer(lwipWriter, tunDev, make([]byte, MTU))
		if err != nil {
			log.Fatalf("copying data failed: %v", err)
		}
	}()

	log.Infof("Running tun2socks")

	osSignals := make(chan os.Signal, 1)
	signal.Notify(osSignals, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGHUP)
	<-osSignals
}
