package tun

import (
	"encoding/binary"

	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const (
	TUNTAP_COMPONENT_ID_0901 = "tap0901"
	TUNTAP_COMPONENT_ID_0801 = "tap0801"
	NETWORK_KEY              = `SYSTEM\CurrentControlSet\Control\Network\{4D36E972-E325-11CE-BFC1-08002BE10318}`
	ADAPTER_KEY              = `SYSTEM\CurrentControlSet\Control\Class\{4D36E972-E325-11CE-BFC1-08002BE10318}`
)

func ctl_code(device_type, function, method, access uint32) uint32 {
	return (device_type << 16) | (access << 14) | (function << 2) | method
}

func tap_control_code(request, method uint32) uint32 {
	return ctl_code(34, request, method, 0)
}

var (
	k32                               = windows.NewLazySystemDLL("kernel32.dll")
	procGetOverlappedResult           = k32.NewProc("GetOverlappedResult")
	TAP_IOCTL_GET_MTU                 = tap_control_code(3, 0)
	TAP_IOCTL_SET_MEDIA_STATUS        = tap_control_code(6, 0)
	TAP_IOCTL_CONFIG_TUN              = tap_control_code(10, 0)
	TAP_WIN_IOCTL_CONFIG_DHCP_MASQ    = tap_control_code(7, 0)
	TAP_WIN_IOCTL_CONFIG_DHCP_SET_OPT = tap_control_code(9, 0)
)

func getTuntapComponentId() (string, error) {
	adapters, err := registry.OpenKey(registry.LOCAL_MACHINE, ADAPTER_KEY, registry.READ)
	if err != nil {
		return "", err
	}

	names, err := adapters.ReadSubKeyNames(0)
	if err != nil {
		return "", err
	}

	for i := 0; i < len(names); i++ {
		if names[i] == "Configuration" || names[i] == "Properties" {
			continue
		}

		adapter, err := registry.OpenKey(registry.LOCAL_MACHINE, fmt.Sprintf("%s\\%s", ADAPTER_KEY, names[i]), registry.READ)
		if err != nil {
			adapter.Close()
			continue
		}

		name, _, err := adapter.GetStringValue("ComponentId")
		if err != nil {
			continue
		}

		if name == TUNTAP_COMPONENT_ID_0901 || name == TUNTAP_COMPONENT_ID_0801 {
			id, _, err := adapter.GetStringValue("NetCfgInstanceId")
			if err == nil {
				log.Printf("device component id: %s", id)
				return id, err
			}
		}
	}

	return "", errors.New("not found component id")
}

func getTuntapName(componentId string) (string, error) {
	adapter, err := registry.OpenKey(registry.LOCAL_MACHINE, fmt.Sprintf("%s\\%s\\Connection", NETWORK_KEY, componentId), registry.READ)
	if err != nil {
		adapter.Close()
		return "", err
	}

	name, _, err := adapter.GetStringValue("Name")
	return name, err
}

func getTuntapComponentIdFromName(adapterName string) (string, error) {
	adapters, err := registry.OpenKey(registry.LOCAL_MACHINE, ADAPTER_KEY, registry.READ)
	if err != nil {
		return "", err
	}

	names, err := adapters.ReadSubKeyNames(0)
	if err != nil {
		return "", err
	}

	for i := 0; i < len(names); i++ {
		if names[i] == "Configuration" || names[i] == "Properties" {
			continue
		}

		adapter, err := registry.OpenKey(registry.LOCAL_MACHINE, fmt.Sprintf("%s\\%s", ADAPTER_KEY, names[i]), registry.READ)
		if err != nil {
			adapter.Close()
			continue
		}

		name, _, err := adapter.GetStringValue("ComponentId")
		if err != nil {
			continue
		}

		if name == TUNTAP_COMPONENT_ID_0901 || name == TUNTAP_COMPONENT_ID_0801 {
			id, _, err := adapter.GetStringValue("NetCfgInstanceId")
			if err == nil {
				_adapterName, err := getTuntapName(id)
				if err == nil {
					if adapterName == _adapterName {
						log.Printf("device component id: %s", id)
						return id, nil
					}
				}
			}
		}
	}

	return "", errors.New("not found component id")
}

func OpenTunDevice(name, addr, gw, mask string, dns []string) (io.ReadWriteCloser, error) {
	var componentId string
	var err error
	if name == "tun1" {
		componentId, err = getTuntapComponentId()
	} else {
		componentId, err = getTuntapComponentIdFromName(name)
	}
	if err != nil {
		return nil, err
	}

	devId, _ := windows.UTF16FromString(fmt.Sprintf(`\\.\Global\%s.tap`, componentId))
	devName, err := getTuntapName(componentId)
	log.Printf("device name: %s", devName)
	// set dhcp with netsh
	cmd := exec.Command("netsh", "interface", "ip", "set", "address", devName, "dhcp")
	cmd.Run()
	cmd = exec.Command("netsh", "interface", "ip", "set", "dns", devName, "dhcp")
	cmd.Run()
	// open
	fd, err := windows.CreateFile(
		&devId[0],
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_SYSTEM|windows.FILE_FLAG_OVERLAPPED,
		//windows.FILE_ATTRIBUTE_SYSTEM,
		0,
	)
	if err != nil {
		return nil, err
	}
	// set addresses with dhcp
	var returnLen uint32
	tunAddr := net.ParseIP(addr).To4()
	tunMask := net.ParseIP(mask).To4()
	gwAddr := net.ParseIP(gw).To4()
	addrParam := append(tunAddr, tunMask...)
	addrParam = append(addrParam, gwAddr...)
	lease := make([]byte, 4)
	binary.BigEndian.PutUint32(lease[:], 86400)
	addrParam = append(addrParam, lease...)
	err = windows.DeviceIoControl(
		fd,
		TAP_WIN_IOCTL_CONFIG_DHCP_MASQ,
		&addrParam[0],
		uint32(len(addrParam)),
		&addrParam[0],
		uint32(len(addrParam)),
		&returnLen,
		nil,
	)
	if err != nil {
		windows.Close(fd)
		return nil, err
	} else {
		log.Printf("set %s with net/mask: %s/%s through DHCP", devName, addr, mask)
	}

	// set dns with dncp
	dnsParam := []byte{6, 4}
	primaryDNS := net.ParseIP(dns[0]).To4()
	dnsParam = append(dnsParam, primaryDNS...)
	if len(dns) >= 2 {
		secondaryDNS := net.ParseIP(dns[1]).To4()
		dnsParam = append(dnsParam, secondaryDNS...)
		dnsParam[1] += 4
	}
	err = windows.DeviceIoControl(
		fd,
		TAP_WIN_IOCTL_CONFIG_DHCP_SET_OPT,
		&dnsParam[0],
		uint32(len(dnsParam)),
		&addrParam[0],
		uint32(len(dnsParam)),
		&returnLen,
		nil,
	)
	if err != nil {
		windows.Close(fd)
		return nil, err
	} else {
		log.Printf("set %s with dns: %s through DHCP", devName, strings.Join(dns, ","))
	}

	// set connect.
	inBuffer := []byte("\x01\x00\x00\x00")
	err = windows.DeviceIoControl(
		fd,
		TAP_IOCTL_SET_MEDIA_STATUS,
		&inBuffer[0],
		uint32(len(inBuffer)),
		&inBuffer[0],
		uint32(len(inBuffer)),
		&returnLen,
		nil,
	)
	if err != nil {
		windows.Close(fd)
		return nil, err
	}
	return newWinTapDev(fd, addr, gw), nil
}

type winTapDev struct {
	// TODO Not sure if a read lock is needed.
	readLock sync.Mutex
	// Write is not allowed concurrent accessing.
	writeLock sync.Mutex

	fd          windows.Handle
	addr        string
	addrIP      net.IP
	gw          string
	gwIP        net.IP
	rBuf        [2048]byte
	wBuf        [2048]byte
	wInitiated  bool
	rOverlapped windows.Overlapped
	wOverlapped windows.Overlapped
}

func newWinTapDev(fd windows.Handle, addr string, gw string) *winTapDev {
	rOverlapped := windows.Overlapped{}
	rEvent, _ := windows.CreateEvent(nil, 0, 0, nil)
	rOverlapped.HEvent = windows.Handle(rEvent)

	wOverlapped := windows.Overlapped{}
	wEvent, _ := windows.CreateEvent(nil, 0, 0, nil)
	wOverlapped.HEvent = windows.Handle(wEvent)

	dev := &winTapDev{
		fd:          fd,
		rOverlapped: rOverlapped,
		wOverlapped: wOverlapped,
		wInitiated:  false,

		addr:   addr,
		addrIP: net.ParseIP(addr).To4(),
		gw:     gw,
		gwIP:   net.ParseIP(gw).To4(),
	}
	return dev
}

func (dev *winTapDev) Read(data []byte) (int, error) {
	dev.readLock.Lock()
	defer dev.readLock.Unlock()

	for {
		var done uint32
		var nr int

		err := windows.ReadFile(dev.fd, dev.rBuf[:], &done, &dev.rOverlapped)
		if err != nil {
			if err != windows.ERROR_IO_PENDING {
				return 0, err
			} else {
				windows.WaitForSingleObject(dev.rOverlapped.HEvent, windows.INFINITE)
				nr, err = getOverlappedResult(dev.fd, &dev.rOverlapped)
				if err != nil {
					return 0, err
				}
			}
		} else {
			nr = int(done)
		}
		if nr > 14 {
			if isStopMarker(dev.rBuf[14:nr], dev.addrIP, dev.gwIP) {
				return 0, errors.New("received stop marker")
			}

			// discard IPv6 packets
			if dev.rBuf[14]&0xf0 == 0x60 {
				log.Printf("ipv6 packet")
				continue
			} else if dev.rBuf[14]&0xf0 == 0x40 {
				if !dev.wInitiated {
					// copy ether header for writing
					copy(dev.wBuf[:], dev.rBuf[6:12])
					copy(dev.wBuf[6:], dev.rBuf[0:6])
					copy(dev.wBuf[12:], dev.rBuf[12:14])
					dev.wInitiated = true
				}
				copy(data, dev.rBuf[14:nr])
				return nr - 14, nil
			}
		}
	}
}

func (dev *winTapDev) Write(data []byte) (int, error) {
	dev.writeLock.Lock()
	defer dev.writeLock.Unlock()

	var done uint32
	var nw int

	payloadL := copy(dev.wBuf[14:], data)
	packetL := payloadL + 14
	err := windows.WriteFile(dev.fd, dev.wBuf[:packetL], &done, &dev.wOverlapped)
	if err != nil {
		if err != windows.ERROR_IO_PENDING {
			return 0, err
		} else {
			windows.WaitForSingleObject(dev.wOverlapped.HEvent, windows.INFINITE)
			nw, err = getOverlappedResult(dev.fd, &dev.wOverlapped)
			if err != nil {
				return 0, err
			}
		}
	} else {
		nw = int(done)
	}
	if nw != packetL {
		return 0, fmt.Errorf("write %d packet (%d bytes payload), return %d", packetL, payloadL, nw)
	} else {
		return payloadL, nil
	}
}

func getOverlappedResult(h windows.Handle, overlapped *windows.Overlapped) (int, error) {
	var n int
	r, _, err := syscall.Syscall6(procGetOverlappedResult.Addr(), 4,
		uintptr(h),
		uintptr(unsafe.Pointer(overlapped)),
		uintptr(unsafe.Pointer(&n)), 1, 0, 0)
	if r == 0 {
		return n, err
	}
	return n, nil
}

func (dev *winTapDev) Close() error {
	log.Printf("close winTap device")
	sendStopMarker(dev.addr, dev.gw)
	return windows.Close(dev.fd)
}
