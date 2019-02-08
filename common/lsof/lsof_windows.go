package lsof

import (
	"errors"

	vnet "v2ray.com/core/common/net"
)

func GetCommandNameBySocket(network string, addr vnet.Address, port vnet.Port) (string, error) {
	return "", errors.New("not implemented")
}
