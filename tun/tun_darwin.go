package tun

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"unsafe"
)

const (
	appleUTUNCtl     = "com.apple.net.utun_control"
	appleCTLIOCGINFO = (0x40000000 | 0x80000000) | ((100 & 0x1fff) << 16) | uint32(byte('N'))<<8 | 3
)

type sockaddrCtl struct {
	scLen      uint8
	scFamily   uint8
	ssSysaddr  uint16
	scID       uint32
	scUnit     uint32
	scReserved [5]uint32
}

type utunDev struct {
	f *os.File

	rBuf [2048]byte
	wBuf [2048]byte
}

func (dev *utunDev) Read(data []byte) (int, error) {
	n, e := dev.f.Read(dev.rBuf[:])
	if n > 0 {
		copy(data, dev.rBuf[4:n])
		n -= 4
	}
	return n, e
}

// one packet, no more than MTU
func (dev *utunDev) Write(data []byte) (int, error) {
	n := copy(dev.wBuf[4:], data)
	return dev.f.Write(dev.wBuf[:n+4])
}

func (dev *utunDev) Close() error {
	return dev.f.Close()
}

var sockaddrCtlSize uintptr = 32

func OpenTunDevice(name, addr, gw, mask string, dns []string) (io.ReadWriteCloser, error) {
	fd, err := syscall.Socket(syscall.AF_SYSTEM, syscall.SOCK_DGRAM, 2)
	if err != nil {
		return nil, err
	}

	var ctlInfo = &struct {
		ctlID   uint32
		ctlName [96]byte
	}{}
	copy(ctlInfo.ctlName[:], []byte(appleUTUNCtl))
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), uintptr(appleCTLIOCGINFO), uintptr(unsafe.Pointer(ctlInfo)))
	if errno != 0 {
		return nil, fmt.Errorf("error in syscall.Syscall(syscall.SYS_IOTL, ...): %v", errno)
	}
	addrP := unsafe.Pointer(&sockaddrCtl{
		scLen:    uint8(sockaddrCtlSize),
		scFamily: syscall.AF_SYSTEM,
		/* #define AF_SYS_CONTROL 2 */
		ssSysaddr: 2,
		scID:      ctlInfo.ctlID,
		scUnit:    0,
	})
	_, _, errno = syscall.RawSyscall(syscall.SYS_CONNECT, uintptr(fd), uintptr(addrP), uintptr(sockaddrCtlSize))
	if errno != 0 {
		return nil, fmt.Errorf("error in syscall.RawSyscall(syscall.SYS_CONNECT, ...): %v", errno)
	}

	var ifName struct {
		name [16]byte
	}
	ifNameSize := uintptr(16)
	_, _, errno = syscall.Syscall6(syscall.SYS_GETSOCKOPT, uintptr(fd),
		2, /* #define SYSPROTO_CONTROL 2 */
		2, /* #define UTUN_OPT_IFNAME 2 */
		uintptr(unsafe.Pointer(&ifName)),
		uintptr(unsafe.Pointer(&ifNameSize)), 0)
	if errno != 0 {
		return nil, fmt.Errorf("error in syscall.Syscall6(syscall.SYS_GETSOCKOPT, ...): %v", errno)
	}
	cmd := exec.Command("ifconfig", string(ifName.name[:ifNameSize-1]), "inet", addr, gw, "netmask", mask, "mtu", "1500", "up")
	err = cmd.Run()
	if err != nil {
		syscall.Close(fd)
		return nil, err
	}

	dev := &utunDev{
		f: os.NewFile(uintptr(fd), string(ifName.name[:ifNameSize-1])),
	}
	copy(dev.wBuf[:], []byte{0, 0, 0, 2})
	return dev, nil
}
