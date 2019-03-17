package route

import (
	"errors"
	"fmt"
	"os/exec"
)

func AddRoute(dest, netmask, gateway string) error {
	out, err := exec.Command("ip", "route", "add", dest+"/32", "via", gateway).Output()
	if err != nil {
		if len(out) != 0 {
			return errors.New(fmt.Sprintf("%v, output: %s", err, out))
		}
		return err
	}
	return nil
}
