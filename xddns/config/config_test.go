package config

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateConfigFilePermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits are not meaningfully testable on windows")
	}

	t.Run("plain file", func(t *testing.T) {
		tests := []struct {
			name    string
			mode    os.FileMode
			wantErr bool
		}{
			{name: "0600 is allowed", mode: 0o600, wantErr: false},
			{name: "0400 is allowed", mode: 0o400, wantErr: false},
			{name: "0644 is rejected (world-readable)", mode: 0o644, wantErr: true},
			{name: "0640 is rejected (group-readable)", mode: 0o640, wantErr: true},
			{name: "0660 is rejected (group-writeable)", mode: 0o660, wantErr: true},
			{name: "0777 is rejected", mode: 0o777, wantErr: true},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "xddns.yaml")
				require.NoError(t, os.WriteFile(path, []byte("updaters: []"), tc.mode))
				require.NoError(t, os.Chmod(path, tc.mode))

				err := validateConfigFilePermissions(path)

				if tc.wantErr {
					require.Error(t, err)
					assert.ErrorIs(t, err, ErrInsecureConfigFile)
					assert.Contains(t, err.Error(), path)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("systemd credential", func(t *testing.T) {
		tests := []struct {
			name    string
			mode    os.FileMode
			wantErr bool
		}{
			{name: "0400 is allowed", mode: 0o400, wantErr: false},
			{name: "0440 is allowed (LoadCredential group-read)", mode: 0o440, wantErr: false},
			{name: "0600 is allowed", mode: 0o600, wantErr: false},
			{name: "0640 is allowed", mode: 0o640, wantErr: false},
			{name: "0444 is rejected (world-readable)", mode: 0o444, wantErr: true},
			{name: "0644 is rejected (world-readable)", mode: 0o644, wantErr: true},
			{name: "0460 is rejected (group-writeable)", mode: 0o460, wantErr: true},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				credDir := t.TempDir()
				t.Setenv("CREDENTIALS_DIRECTORY", credDir)

				path := filepath.Join(credDir, "xddns.yaml")
				require.NoError(t, os.WriteFile(path, []byte("updaters: []"), tc.mode))
				require.NoError(t, os.Chmod(path, tc.mode))

				err := validateConfigFilePermissions(path)

				if tc.wantErr {
					assert.ErrorIs(t, err, ErrInsecureConfigFile)
				} else {
					assert.NoError(t, err)
				}
			})
		}
	})

	t.Run("path outside CREDENTIALS_DIRECTORY", func(t *testing.T) {
		credDir := t.TempDir()
		t.Setenv("CREDENTIALS_DIRECTORY", credDir)

		otherDir := t.TempDir()
		path := filepath.Join(otherDir, "xddns.yaml")

		require.NoError(t, os.WriteFile(path, []byte("updaters: []"), 0o440))
		require.NoError(t, os.Chmod(path, 0o440))

		err := validateConfigFilePermissions(path)

		assert.ErrorIs(t, err, ErrInsecureConfigFile)
	})

	t.Run("CREDENTIALS_DIRECTORY unset", func(t *testing.T) {
		t.Setenv("CREDENTIALS_DIRECTORY", "")

		dir := t.TempDir()
		path := filepath.Join(dir, "xddns.yaml")
		require.NoError(t, os.WriteFile(path, []byte("updaters: []"), 0o440))
		require.NoError(t, os.Chmod(path, 0o440))

		err := validateConfigFilePermissions(path)

		assert.ErrorIs(t, err, ErrInsecureConfigFile)
	})

	t.Run("nonexistent file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "does-not-exist.yaml")

		err := validateConfigFilePermissions(path)

		assert.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestIsSystemdCredential(t *testing.T) {
	t.Run("CREDENTIALS_DIRECTORY unset", func(t *testing.T) {
		t.Setenv("CREDENTIALS_DIRECTORY", "")
		assert.False(t, isSystemdCredential("/run/credentials/xddns.service/xddns.yaml"))
	})

	t.Run("file inside CREDENTIALS_DIRECTORY", func(t *testing.T) {
		credDir := t.TempDir()
		t.Setenv("CREDENTIALS_DIRECTORY", credDir)

		path := filepath.Join(credDir, "xddns.yaml")
		assert.True(t, isSystemdCredential(path))
	})
}
