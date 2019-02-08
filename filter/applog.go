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
	if v := packet.PeekIPVersion(buf); v == packet.IPVERSION_4 {
		network := packet.PeekProtocol(buf)
		// TODO log for udp packets
		if network == "tcp" {
			// TODO only log once for each established tcp connection
			if packet.IsSYNSegment(buf) {
				srcAddr := packet.PeekSourceAddress(buf)
				srcPort := packet.PeekSourcePort(buf)
				destAddr := packet.PeekDestinationAddress(buf)
				destPort := packet.PeekDestinationPort(buf)
				// Don't block
				go func() {
					name, err := lsof.GetCommandNameBySocket(network, srcAddr, srcPort)
					if err != nil {
						log.Printf("failed to get app information by socket %v:%v:%v", network, srcAddr, srcPort)
					} else {
						log.Printf("|%v| is connecting %v:%v:%v", name, network, destAddr, destPort)
					}
				}()
			}
		}
	}
	return w.writer.Write(buf)
}
