package tun

import (
	"fmt"
	"io"
	"net"
	"os/exec"

	"github.com/songgao/water"
)

func OpenTunDevice(name, addr, gw, mask, id string, dns []string, persist bool) (io.ReadWriteCloser, error) {
	// The following commands to configure the TAP adapter require elevation. Don't fail if the we are
	// not running as admin; the device can be configured beforehand.
	// Set the device IP address and network.
	cmd := exec.Command("netsh", "interface", "ip", "set", "address", name, "static", addr, mask)
	cmd.Run()
	// Set the device primary DNS resolver.
	cmd = exec.Command("netsh", "interface", "ip", "set", "dns", name, "static", dns[0])
	cmd.Run()
	if len(dns) >= 2 {
		// Add a secondary DNS resolver.
		cmd = exec.Command("netsh", "interface", "ip", "add", "dns", name, dns[1], "index=2")
		cmd.Run()
	}
	prefix, _ := net.IPMask(net.ParseIP(mask).To4()).Size()
	network := fmt.Sprintf("%v/%v", addr, prefix)
	return water.New(water.Config{
		DeviceType: water.TUN,
		PlatformSpecificParams: water.PlatformSpecificParams{
			ComponentID:   id,
			InterfaceName: name,
			Network:       network,
		},
	})
}
