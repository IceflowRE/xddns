//go:build !windows

package config

import (
	"errors"
	"fmt"
	"os"
)

var errNilConfigFileHandle = errors.New("config file handle is nil")

func validatePlatformConfigFilePermissions(file *os.File, path string) error {
	if file == nil {
		return errNilConfigFileHandle
	}

	info, err := file.Stat()
	if err != nil {
		return err
	}

	mode := info.Mode().Perm()
	if isDirInSystemdCredential(path) {
		// systemd credential files are set to 0440 by default.
		if mode&0o0037 != 0 {
			return fmt.Errorf("%w (%04o): %q. Unexpected permissions for a systemd credential file", ErrInsecureConfigFile, mode, path)
		}

		return nil
	}

	if mode&0o0077 != 0 {
		return fmt.Errorf("%w (%04o): %q. It should be readable and writeable only by the owner (0600)", ErrInsecureConfigFile, mode, path)
	}

	return nil
}
