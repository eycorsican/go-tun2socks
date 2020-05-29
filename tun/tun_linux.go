package tun

import (
	"fmt"
	"github.com/eycorsican/go-tun2socks/core"
	"github.com/songgao/water"
	"io"
	"net"
	"os"
	"syscall"
	"time"
)

func OpenTunDevice(name, addr, gw, mask string, dnsServers []string, persist bool) (io.ReadWriteCloser, error) {
	cfg := water.Config{
		DeviceType: water.TUN,
	}
	cfg.Name = name
	cfg.Persist = persist
	tunDev, err := water.New(cfg)
	if err != nil {
		return nil, err
	}
	name = tunDev.Name()
	return tunDev, nil
}

func OpenTunDeviceByDomainSocket(sockpath, name, addr, gw, mask string, dnsServers []string, persist bool) (io.ReadWriteCloser, error) {
	cfg := water.Config{
		DeviceType: water.TUN,
	}
	cfg.Name = name
	cfg.Persist = persist
	tunDev, err := openDevByDomainSocket(cfg, sockpath)
	if err != nil {
		return nil, err
	}
	return tunDev, nil
}

//copied from https://golang.org/src/syscall/syscall_unix_test.go
func openDevByDomainSocket(config water.Config, sockpath string) (ifce *water.Interface, err error) {
	//fmt.Println("sockpath=",sockpath)

	os.RemoveAll(sockpath)
	l, err := net.Listen("unix", sockpath)
	if err != nil {
		//t.Fatalf("unexpected FileConn type; expected UnixConn, got %T", c)
		//return nil,errors.New(fmt.Sprintf("unexpected FileConn type; expected UnixConn, got %T", c))
		return nil,err
	}

	itf := &water.Interface{

	}

	go func() {
		conn, err := l.Accept()
		if err != nil {
			fmt.Println(err)
			return
		}
		uc, ok := conn.(*net.UnixConn)
		if !ok {
			fmt.Println(fmt.Sprintf("unexpected FileConn type; expected UnixConn, got %T", conn))
		}
		buf := make([]byte, 32) // expect 1 byte
		oob := make([]byte, 32) // expect 24 bytes
		closeUnix := time.AfterFunc(15*time.Second, func() {
			//t.Logf("timeout reading from unix socket")
			uc.Close()
			//return nil,errors.New("timeout reading from unix socket")
			fmt.Println("timeout reading from unix socket")
		})
		_, oobn, _, _, err := uc.ReadMsgUnix(buf, oob)
		if err != nil {
			//t.Fatalf("ReadMsgUnix: %v", err)
			//return nil,errors.New(fmt.Sprintf("ReadMsgUnix: %v", err))
			fmt.Println("ReadMsgUnix: ", err)
			return
		}
		closeUnix.Stop()

		scms, err := syscall.ParseSocketControlMessage(oob[:oobn])
		if err != nil {
			//t.Fatalf("ParseSocketControlMessage: %v", err)
			//return nil,errors.New(fmt.Sprintf("ParseSocketControlMessage: %v", err))
			fmt.Println("ParseSocketControlMessage: ", err)
			return
		}
		if len(scms) != 1 {
			//t.Fatalf("expected 1 SocketControlMessage; got scms = %#v", scms)
			//return nil,errors.New(fmt.Sprintf("expected 1 SocketControlMessage; got scms = %#v", scms))
			fmt.Println("expected 1 SocketControlMessage; got scms = ", scms)
			return
		}
		scm := scms[0]
		gotFds, err := syscall.ParseUnixRights(&scm)
		if err != nil {
			//t.Fatalf("syscall.ParseUnixRights: %v", err)
			//return nil,errors.New(fmt.Sprintf("syscall.ParseUnixRights: %v", err))
			fmt.Println("syscall.ParseUnixRights: ", err)
			return
		}
		if len(gotFds) != 1 {
			//t.Fatalf("wanted 1 fd; got %#v", gotFds)
			//return nil,errors.New(fmt.Sprintf("wanted 1 fd; got %#v", gotFds))
			fmt.Println("wanted 1 fd; got ", gotFds)
			return
		}
		fdInt := gotFds[0]

		itf.ReadWriteCloser = os.NewFile(uintptr(fdInt), "tun0")

		lwipWriter := core.NewLWIPStack().(io.Writer)
		io.CopyBuffer(lwipWriter, itf, make([]byte, 1500))
	}()

	return itf,nil
}
