package lsof

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"

	vnet "v2ray.com/core/common/net"
)

func GetCommandNameBySocket(network string, addr vnet.Address, port vnet.Port) (string, error) {
	// See `lsof -F?` for help
	out, err := exec.Command("lsof", "-n", "-Fc", fmt.Sprintf("-i%s@%s:%s", network, addr, port)).Output()
	if err != nil {
		if len(out) != 0 {
			return "", errors.New(fmt.Sprintf("%v, output: %s", err, out))
		}
		return "", err
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		// There may be multiple candidate
		// sockets in the list, just take
		// the first one for simplicity.
		if strings.HasPrefix(line, "c") {
			return line[1:len(line)], nil
		}
	}
	return "", errors.New("not found")
}
