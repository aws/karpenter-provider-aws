package cli

import (
	"os/user"
)

// IsRunningAsRoot returns true if the current user has UID 0.
func IsRunningAsRoot() (bool, error) {
	usr, err := user.Current()
	if err != nil {
		return false, err
	}
	return usr.Uid == "0", nil
}
