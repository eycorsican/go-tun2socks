package tun

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"io"
	"net"
	"unsafe"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/sys/windows"

	"golang.zx2c4.com/wireguard/tun"
	"golang.zx2c4.com/wireguard/windows/tunnel/winipcfg"
)

func init() {
	if err := checkForWow64(); err != nil {
		panic(err)
	}
}

func checkForWow64() error {
	var b bool
	err := windows.IsWow64Process(windows.CurrentProcess(), &b)
	if err != nil {
		return fmt.Errorf("Unable to determine whether the process is running under WOW64: %v", err)
	}
	if b {
		return fmt.Errorf("You must use the 64-bit version of WireGuard on this computer.")
	}
	return nil
}

func OpenTunDevice(name, addr, gw, mask string, dns []string, persist bool) (io.ReadWriteCloser, error) {
	dev := tunDevice{}
	device, err := tun.CreateTUNWithRequestedGUID(name, determineGUID(name), 1500)
	if err != nil {
		return nil, err
	}
	dev.NativeTun = device.(*tun.NativeTun)

	if net.ParseIP(addr).To4() != nil {
		err = dev.setInterfaceAddress4(addr, mask, gw)
	} else {
		err = dev.setInterfaceAddress6(addr, mask, gw)
	}
	if err != nil {
		dev.Close()
		return nil, err
	}

	if err = dev.setDNS(dns); err != nil {
		dev.Close()
		return nil, err
	}

	return dev, nil
}

func determineGUID(name string) *windows.GUID {
	b := make([]byte, unsafe.Sizeof(windows.GUID{}))
	if _, err := io.ReadFull(hkdf.New(md5.New, []byte(name), nil, nil), b); err != nil {
		return nil
	}
	return (*windows.GUID)(unsafe.Pointer(&b[0]))
}

type tunDevice struct {
	*tun.NativeTun
}

func (d tunDevice) Read(b []byte) (int, error) {
	return d.NativeTun.Read(b, 0)
}

func (d tunDevice) Write(b []byte) (int, error) {
	return d.NativeTun.Write(b, 0)
}

//https://github.com/WireGuard/wireguard-windows/blob/ef8d4f03bbb6e407bc4470b2134a9ab374155633/tunnel/addressconfig.go#L22-L58
func cleanupAddressesOnDisconnectedInterfaces(family winipcfg.AddressFamily, addresses []net.IPNet) {
	if len(addresses) == 0 {
		return
	}
	includedInAddresses := func(a net.IPNet) bool {
		// TODO: this makes the whole algorithm O(n^2). But we can't stick net.IPNet in a Go hashmap. Bummer!
		for _, addr := range addresses {
			ip := addr.IP
			if ip4 := ip.To4(); ip4 != nil {
				ip = ip4
			}
			mA, _ := addr.Mask.Size()
			mB, _ := a.Mask.Size()
			if bytes.Equal(ip, a.IP) && mA == mB {
				return true
			}
		}
		return false
	}
	interfaces, err := winipcfg.GetAdaptersAddresses(family, winipcfg.GAAFlagDefault)
	if err != nil {
		return
	}
	for _, iface := range interfaces {
		if iface.OperStatus == winipcfg.IfOperStatusUp {
			continue
		}
		for address := iface.FirstUnicastAddress; address != nil; address = address.Next {
			ip := address.Address.IP()
			ipnet := net.IPNet{IP: ip, Mask: net.CIDRMask(int(address.OnLinkPrefixLength), 8*len(ip))}
			if includedInAddresses(ipnet) {
				iface.LUID.DeleteIPAddress(ipnet)
			}
		}
	}
}

//https://github.com/WireGuard/wireguard-windows/blob/ef8d4f03bbb6e407bc4470b2134a9ab374155633/tunnel/addressconfig.go#L60-L168
func (d tunDevice) setInterfaceAddress4(addr, mask, gateway string) error {
	luid := winipcfg.LUID(d.NativeTun.LUID())

	addresses := append([]net.IPNet{}, net.IPNet{
		IP:   net.ParseIP(addr).To4(),
		Mask: net.IPMask(net.ParseIP(mask).To4()),
	})

	err := luid.SetIPAddressesForFamily(windows.AF_INET, addresses)
	if err == windows.ERROR_OBJECT_ALREADY_EXISTS {
		cleanupAddressesOnDisconnectedInterfaces(windows.AF_INET, addresses)
		err = luid.SetIPAddressesForFamily(windows.AF_INET, addresses)
	}

	return err
}

func (d tunDevice) setInterfaceAddress6(addr, mask, gateway string) error {
	luid := winipcfg.LUID(d.NativeTun.LUID())

	addresses := append([]net.IPNet{}, net.IPNet{
		IP:   net.ParseIP(addr).To16(),
		Mask: net.IPMask(net.ParseIP(mask).To16()),
	})

	err := luid.SetIPAddressesForFamily(windows.AF_INET6, addresses)
	if err == windows.ERROR_OBJECT_ALREADY_EXISTS {
		cleanupAddressesOnDisconnectedInterfaces(windows.AF_INET6, addresses)
		err = luid.SetIPAddressesForFamily(windows.AF_INET6, addresses)
	}

	return err
}

func (d tunDevice) setDNS(dns []string) error {
	luid := winipcfg.LUID(d.NativeTun.LUID())

	ip4 := make([]net.IP, 0, 1)
	ip6 := make([]net.IP, 0, 1)

	for _, name := range dns {
		ip := net.ParseIP(name)
		if ipv4 := ip.To4(); ipv4 != nil {
			ip4 = append(ip4, ipv4)
			continue
		}
		if ipv6 := ip.To16(); ipv6 != nil {
			ip6 = append(ip6, ipv6)
		}
	}

	e1 := luid.SetDNSForFamily(windows.AF_INET, ip4)
	e2 := luid.SetDNSForFamily(windows.AF_INET6, ip6)
	if e1 != nil || e2 != nil {
		return fmt.Errorf("ipv4 dns error: %v, ipv6 dns error: %v", e1, e2)
	}

	return nil
}
