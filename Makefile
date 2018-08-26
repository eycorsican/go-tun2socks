GOCMD=go
XGOCMD=xgo
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOCLEAN=$(GOCMD) clean
BUILDDIR=$(shell pwd)/build
CMDDIR=$(shell pwd)/cmd/tun2socks
PROGRAM=tun2socks
LWIP_DIR=$(shell pwd)/lwip
LWIP_SRC_DIR=$(LWIP_DIR)/src
LWIP_INCLUDE_DIR=$(LWIP_SRC_DIR)/include
LWIP_HEADERS_DIR=$(LWIP_INCLUDE_DIR)/lwip

CORE_FILES=$(LWIP_SRC_DIR)/core/init.c \
    $(LWIP_SRC_DIR)/core/def.c \
    $(LWIP_SRC_DIR)/core/dns.c \
    $(LWIP_SRC_DIR)/core/inet_chksum.c \
    $(LWIP_SRC_DIR)/core/ip.c \
    $(LWIP_SRC_DIR)/core/mem.c \
    $(LWIP_SRC_DIR)/core/memp.c \
    $(LWIP_SRC_DIR)/core/netif.c \
    $(LWIP_SRC_DIR)/core/pbuf.c \
    $(LWIP_SRC_DIR)/core/raw.c \
    $(LWIP_SRC_DIR)/core/stats.c \
    $(LWIP_SRC_DIR)/core/sys.c \
    $(LWIP_SRC_DIR)/core/tcp.c \
    $(LWIP_SRC_DIR)/core/tcp_in.c \
    $(LWIP_SRC_DIR)/core/tcp_out.c \
    $(LWIP_SRC_DIR)/core/timeouts.c \
    $(LWIP_SRC_DIR)/core/udp.c

CORE_4_FILES=$(LWIP_SRC_DIR)/core/ipv4/autoip.c \
    $(LWIP_SRC_DIR)/core/ipv4/dhcp.c \
    $(LWIP_SRC_DIR)/core/ipv4/etharp.c \
    $(LWIP_SRC_DIR)/core/ipv4/icmp.c \
    $(LWIP_SRC_DIR)/core/ipv4/igmp.c \
    $(LWIP_SRC_DIR)/core/ipv4/ip4_frag.c \
    $(LWIP_SRC_DIR)/core/ipv4/ip4.c \
    $(LWIP_SRC_DIR)/core/ipv4/ip4_addr.c

CORE_6_FILES=$(LWIP_SRC_DIR)/core/ipv6/dhcp6.c \
    $(LWIP_SRC_DIR)/core/ipv6/ethip6.c \
    $(LWIP_SRC_DIR)/core/ipv6/icmp6.c \
    $(LWIP_SRC_DIR)/core/ipv6/inet6.c \
    $(LWIP_SRC_DIR)/core/ipv6/ip6.c \
    $(LWIP_SRC_DIR)/core/ipv6/ip6_addr.c \
    $(LWIP_SRC_DIR)/core/ipv6/ip6_frag.c \
    $(LWIP_SRC_DIR)/core/ipv6/mld6.c \
    $(LWIP_SRC_DIR)/core/ipv6/nd6.c

CUSTOM_SRC_FILES=$(LWIP_SRC_DIR)/custom/sys_arch.c
CUSTOM_INCLUDE_FILES=$(LWIP_SRC_DIR)/custom/arch
CUSTOM_HEADER_FILES=$(LWIP_SRC_DIR)/custom/lwipopts.h

all: build

build:
	mkdir -p $(BUILDDIR)
	cp $(CORE_FILES) $(LWIP_DIR)/
	cp $(CORE_4_FILES) $(LWIP_DIR)/
	cp $(CORE_6_FILES) $(LWIP_DIR)/
	cp $(CUSTOM_SRC_FILES) $(LWIP_DIR)/
	cp -r $(CUSTOM_INCLUDE_FILES) $(LWIP_INCLUDE_DIR)/
	cp -r $(CUSTOM_HEADER_FILES) $(LWIP_HEADERS_DIR)/

	cd $(CMDDIR) && $(GOBUILD) -o $(BUILDDIR)/$(PROGRAM) -v

	rm -rf $(LWIP_DIR)/*.c
	rm -rf $(LWIP_INCLUDE_DIR)/arch
	rm -rf $(LWIP_HEADERS_DIR)/lwipopts.h

xbuild:
	mkdir -p $(BUILDDIR)
	cp $(CORE_FILES) $(LWIP_DIR)/
	cp $(CORE_4_FILES) $(LWIP_DIR)/
	cp $(CORE_6_FILES) $(LWIP_DIR)/
	cp $(CUSTOM_SRC_FILES) $(LWIP_DIR)/
	cp -r $(CUSTOM_INCLUDE_FILES) $(LWIP_INCLUDE_DIR)/
	cp -r $(CUSTOM_HEADER_FILES) $(LWIP_HEADERS_DIR)/

	cd $(BUILDDIR) && $(XGOCMD) --targets=*/* $(CMDDIR)

	rm -rf $(LWIP_DIR)/*.c
	rm -rf $(LWIP_INCLUDE_DIR)/arch
	rm -rf $(LWIP_HEADERS_DIR)/lwipopts.h

copy:
	cp $(CORE_FILES) $(LWIP_DIR)/
	cp $(CORE_4_FILES) $(LWIP_DIR)/
	cp $(CORE_6_FILES) $(LWIP_DIR)/
	cp $(CUSTOM_SRC_FILES) $(LWIP_DIR)/
	cp -r $(CUSTOM_INCLUDE_FILES) $(LWIP_INCLUDE_DIR)/
	cp -r $(CUSTOM_HEADER_FILES) $(LWIP_HEADERS_DIR)/

clean:
	$(GOCLEAN) -cache
	rm -rf $(BUILDDIR)
	rm -rf $(LWIP_DIR)/*.c
	rm -rf $(LWIP_INCLUDE_DIR)/arch
	rm -rf $(LWIP_HEADERS_DIR)/lwipopts.h
