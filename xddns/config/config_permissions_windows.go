//go:build windows

package config

import (
	"errors"
	"fmt"
	"os"
	"unsafe"

	"golang.org/x/sys/windows"
)

var errNilConfigFileHandle = errors.New("config file handle is nil")

func validatePlatformConfigFilePermissions(file *os.File, path string) error {
	if file == nil {
		return errNilConfigFileHandle
	}

	securityDescriptor, err := windows.GetSecurityInfo(
		windows.Handle(file.Fd()),
		windows.SE_FILE_OBJECT,
		windows.OWNER_SECURITY_INFORMATION|windows.DACL_SECURITY_INFORMATION,
	)
	if err != nil {
		return fmt.Errorf("%w: unable to read Windows ACL for %q: %w", ErrInsecureConfigFile, path, err)
	}
	if securityDescriptor == nil {
		return fmt.Errorf("%w: %q has no Windows security descriptor", ErrInsecureConfigFile, path)
	}

	token := windows.GetCurrentProcessToken()
	defer func() {
		_ = token.Close()
	}()

	tokenUser, err := token.GetTokenUser()
	if err != nil {
		return fmt.Errorf("%w: unable to identify the current Windows user: %w", ErrInsecureConfigFile, err)
	}
	systemSID, err := windows.CreateWellKnownSid(windows.WinLocalSystemSid)
	if err != nil {
		return fmt.Errorf("%w: unable to identify the Windows system account: %w", ErrInsecureConfigFile, err)
	}

	owner, _, err := securityDescriptor.Owner()
	if err != nil {
		return fmt.Errorf("%w: unable to read the owner of %q: %w", ErrInsecureConfigFile, path, err)
	}
	if owner == nil || (!owner.Equals(tokenUser.User.Sid) && !owner.Equals(systemSID)) {
		return fmt.Errorf("%w: %q is not owned by the current Windows user or SYSTEM", ErrInsecureConfigFile, path)
	}

	dacl, _, err := securityDescriptor.DACL()
	if err != nil {
		return fmt.Errorf("%w: unable to read Windows ACL for %q: %w", ErrInsecureConfigFile, path, err)
	}
	if dacl == nil {
		return fmt.Errorf("%w: %q has an unrestricted Windows ACL", ErrInsecureConfigFile, path)
	}

	administratorsSID, err := windows.CreateWellKnownSid(windows.WinBuiltinAdministratorsSid)
	if err != nil {
		return fmt.Errorf("%w: unable to identify the Windows administrators group: %w", ErrInsecureConfigFile, err)
	}

	return validateConfigFileACL(dacl, owner, tokenUser.User.Sid, systemSID, administratorsSID, path)
}

func validateConfigFileACL(dacl *windows.ACL, owner *windows.SID, user *windows.SID, system *windows.SID, administrators *windows.SID, path string) error {
	var err error
	for index := range int(dacl.AceCount) {
		var ace *windows.ACCESS_ALLOWED_ACE
		err = windows.GetAce(dacl, uint32(index), &ace)
		if err != nil {
			return fmt.Errorf("%w: unable to inspect Windows ACL for %q: %w", ErrInsecureConfigFile, path, err)
		}
		if ace.Header.AceFlags&windows.INHERIT_ONLY_ACE != 0 {
			continue
		}

		switch ace.Header.AceType {
		case windows.ACCESS_DENIED_ACE_TYPE:
			continue
		case windows.ACCESS_ALLOWED_ACE_TYPE:
			aceSID := (*windows.SID)(unsafe.Pointer(&ace.SidStart)) //nolint:gosec // Windows ACL API exposes the SID through SidStart.
			// Only the file owner, current user, SYSTEM, and local administrators are trusted to access configuration files.
			if aceSID.Equals(owner) || aceSID.Equals(user) || aceSID.Equals(system) || aceSID.Equals(administrators) {
				continue
			}
		default:
			return fmt.Errorf("%w: %q has an ACL entry granting access to another account", ErrInsecureConfigFile, path)
		}

		return fmt.Errorf("%w: %q has an ACL entry granting access to another account", ErrInsecureConfigFile, path)
	}

	return nil
}
