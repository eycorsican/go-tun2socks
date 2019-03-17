package filter

import (
	"io"
	"net"

	"github.com/eycorsican/go-tun2socks/common/log"
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
	if network != "tcp" {
		return w.writer.Write(buf)
	}
	if network == "tcp" && !packet.IsSYNSegment(buf) {
		return w.writer.Write(buf)
	}
	srcAddr := net.IP(append([]byte(nil), []byte(packet.PeekSourceAddress(buf))...))
	srcPort := packet.PeekSourcePort(buf)
	destAddr := net.IP(append([]byte(nil), []byte(packet.PeekDestinationAddress(buf))...))
	destPort := packet.PeekDestinationPort(buf)
	go func() {
		name, err := lsof.GetCommandNameBySocket(network, srcAddr.String(), srcPort)
		if err != nil {
			name = "unknown process"
		}
		log.Infof("[%v] is connecting %v:%v:%v", name, network, destAddr.String(), destPort)
	}()
	return w.writer.Write(buf)
}
