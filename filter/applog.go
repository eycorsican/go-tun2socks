package filter

import (
	"io"
	"log"

	"github.com/eycorsican/go-tun2socks/common/lsof"
	"github.com/eycorsican/go-tun2socks/common/packet"
)

// A filter to log information about the application which sends this data segment.
type applogFilter struct {
	writer io.Writer
}

func NewApplogFilter(w io.Writer) Filter {
	return &applogFilter{writer: w}
}

func (w *applogFilter) Write(buf []byte) (int, error) {
	if v := packet.PeekIPVersion(buf); v != packet.IPVERSION_4 {
		return w.writer.Write(buf)
	}
	network := packet.PeekProtocol(buf)
	// TODO may be overwhelmed by udp logs, better skip udp?
	if network != "tcp" && network != "udp" {
		return w.writer.Write(buf)
	}
	if network == "tcp" && !packet.IsSYNSegment(buf) {
		return w.writer.Write(buf)
	}

	srcAddr := packet.PeekSourceAddress(buf)
	srcPort := packet.PeekSourcePort(buf)
	destAddr := packet.PeekDestinationAddress(buf)
	destPort := packet.PeekDestinationPort(buf)
	go func() {
		name, err := lsof.GetCommandNameBySocket(network, srcAddr, srcPort)
		if err != nil {
			name = "unknown process"
		}
		log.Printf("[%v] is connecting %v:%v:%v", name, network, destAddr, destPort)
	}()
	return w.writer.Write(buf)
}
