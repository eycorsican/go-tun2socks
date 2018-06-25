GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOCLEAN=$(GOCMD) clean
BUILDDIR=$(shell pwd)/build
CMDDIR=$(shell pwd)/cmd/tun2socks
PROGRAM=tun2socks
LWIPDIR=$(shell pwd)/lwip
LWIPSRCDIR=$(LWIPDIR)/src

all: run

run:
	cp $(LWIPSRCDIR)/*.c $(LWIPDIR)/
	cd $(CMDDIR) && $(GORUN)

build:
	mkdir -p $(BUILDDIR)
	cp $(LWIPSRCDIR)/*.c $(LWIPDIR)/
	cd $(CMDDIR) && $(GOBUILD) -o $(BUILDDIR)/$(PROGRAM) -v
	rm -rf $(LWIPDIR)/*.c

copy:
	cp $(LWIPSRCDIR)/*.c $(LWIPDIR)/

clean:
	$(GOCLEAN) -cache
	rm -rf $(LWIPDIR)/*.c
	rm -rf $(BUILDDIR)
