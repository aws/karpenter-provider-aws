package system

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

// Returns whether FIPS module is both installed an enabled on the system
//
//	ipsInstalled, fipsEnabled, err := getFipsInfo()
func GetFipsInfo() (bool, bool, error) {
	fipsEnabledBytes, err := os.ReadFile("/proc/sys/crypto/fips_enabled")
	if errors.Is(err, os.ErrNotExist) {
		return false, false, nil
	} else if err != nil {
		return false, false, err
	}
	fipsEnabledInt, err := strconv.Atoi(strings.Trim(string(fipsEnabledBytes), " \n\t"))
	if err != nil {
		return true, false, err
	}
	return true, fipsEnabledInt == 1, nil
}
