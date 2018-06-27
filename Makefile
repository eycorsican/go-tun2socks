GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOCLEAN=$(GOCMD) clean
BUILDDIR=$(shell pwd)/build
CMDDIR=$(shell pwd)/cmd/tun2socks
PROGRAM=tun2socks
LWIPDIR=$(shell pwd)/lwip
LWIPSRCDIR=$(LWIPDIR)/src
LWIPHEADERSDIR=$(LWIPDIR)/src/include/lwip
OS=$(shell uname -s)

COREFILES=$(LWIPSRCDIR)/core/init.c \
    $(LWIPSRCDIR)/core/def.c \
    $(LWIPSRCDIR)/core/dns.c \
    $(LWIPSRCDIR)/core/inet_chksum.c \
    $(LWIPSRCDIR)/core/ip.c \
    $(LWIPSRCDIR)/core/mem.c \
    $(LWIPSRCDIR)/core/memp.c \
    $(LWIPSRCDIR)/core/netif.c \
    $(LWIPSRCDIR)/core/pbuf.c \
    $(LWIPSRCDIR)/core/raw.c \
    $(LWIPSRCDIR)/core/stats.c \
    $(LWIPSRCDIR)/core/sys.c \
    $(LWIPSRCDIR)/core/tcp.c \
    $(LWIPSRCDIR)/core/tcp_in.c \
    $(LWIPSRCDIR)/core/tcp_out.c \
    $(LWIPSRCDIR)/core/timeouts.c \
    $(LWIPSRCDIR)/core/udp.c

CORE4FILES=$(LWIPSRCDIR)/core/ipv4/autoip.c \
    $(LWIPSRCDIR)/core/ipv4/dhcp.c \
    $(LWIPSRCDIR)/core/ipv4/etharp.c \
    $(LWIPSRCDIR)/core/ipv4/icmp.c \
    $(LWIPSRCDIR)/core/ipv4/igmp.c \
    $(LWIPSRCDIR)/core/ipv4/ip4_frag.c \
    $(LWIPSRCDIR)/core/ipv4/ip4.c \
    $(LWIPSRCDIR)/core/ipv4/ip4_addr.c

CORE6FILES=$(LWIPSRCDIR)/core/ipv6/dhcp6.c \
    $(LWIPSRCDIR)/core/ipv6/ethip6.c \
    $(LWIPSRCDIR)/core/ipv6/icmp6.c \
    $(LWIPSRCDIR)/core/ipv6/inet6.c \
    $(LWIPSRCDIR)/core/ipv6/ip6.c \
    $(LWIPSRCDIR)/core/ipv6/ip6_addr.c \
    $(LWIPSRCDIR)/core/ipv6/ip6_frag.c \
    $(LWIPSRCDIR)/core/ipv6/mld6.c \
    $(LWIPSRCDIR)/core/ipv6/nd6.c

CUSTOMFILES=$(LWIPSRCDIR)/custom/sys_arch.c

CUSTOMHEADERFILES=$(LWIPSRCDIR)/custom/arch \
    $(LWIPSRCDIR)/custom/lwipopts.h

all: build

build:
	mkdir -p $(BUILDDIR)
	cp $(COREFILES) $(LWIPDIR)/
	cp $(CORE4FILES) $(LWIPDIR)/
	cp $(CORE6FILES) $(LWIPDIR)/
	cp $(CUSTOMFILES) $(LWIPDIR)/
	cp -r $(CUSTOMHEADERFILES) $(LWIPHEADERSDIR)/

ifeq ($(OS), Darwin)
	cd $(CMDDIR) && CGO_CPPFLAGS='-DDARWIN=1' $(GOBUILD) -o $(BUILDDIR)/$(PROGRAM) -v
else ifeq ($(OS), Linux)
	cd $(CMDDIR) && CGO_CPPFLAGS='-DLINUX=1' $(GOBUILD) -o $(BUILDDIR)/$(PROGRAM) -v
endif

	rm -rf $(LWIPDIR)/*.c
	rm -rf $(LWIPHEADERSDIR)/arch
	rm -rf $(LWIPHEADERSDIR)/lwipopts.h

copy:
	cp $(COREFILES) $(LWIPDIR)/
	cp $(CORE4FILES) $(LWIPDIR)/
	cp $(CORE6FILES) $(LWIPDIR)/
	cp $(CUSTOMFILES) $(LWIPDIR)/
	cp -r $(CUSTOMHEADERFILES) $(LWIPHEADERSDIR)/

clean:
	$(GOCLEAN) -cache
	rm -rf $(BUILDDIR)
	rm -rf $(LWIPDIR)/*.c
	rm -rf $(LWIPHEADERSDIR)/arch
	rm -rf $(LWIPHEADERSDIR)/lwipopts.h
