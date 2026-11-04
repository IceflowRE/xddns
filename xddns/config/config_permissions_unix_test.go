//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validateConfigFilePermissions(path string) error {
	file, err := os.Open(path) //nolint:gosec
	if err != nil {
		return err
	}
	defer func() {
		_ = file.Close()
	}()

	return validatePlatformConfigFilePermissions(file, path)
}

func TestValidateConfigFilePermissionsUsesCredentialsDirectory(t *testing.T) {
	credDir := t.TempDir()
	t.Setenv("CREDENTIALS_DIRECTORY", credDir)

	path := filepath.Join(credDir, "xddns.yaml")
	require.NoError(t, os.WriteFile(path, []byte("updaters: []"), 0o440))
	require.NoError(t, os.Chmod(path, 0o440))

	assert.NoError(t, validateConfigFilePermissions(path))
}

func TestValidateConfigFilePermissionsRejectsCredentialModeOutsideCredentialsDirectory(t *testing.T) {
	credDir := t.TempDir()
	t.Setenv("CREDENTIALS_DIRECTORY", credDir)

	dir := t.TempDir()
	path := filepath.Join(dir, "xddns.yaml")
	require.NoError(t, os.WriteFile(path, []byte("updaters: []"), 0o440))
	require.NoError(t, os.Chmod(path, 0o440))

	assert.ErrorIs(t, validateConfigFilePermissions(path), ErrInsecureConfigFile)
}
