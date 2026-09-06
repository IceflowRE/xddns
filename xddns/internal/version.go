package internal

import (
	"runtime/debug"
)

var version = ""

// Version returns the current version of the application.
// If the version is not set, it attempts to retrieve it from the build information. If that fails, it returns "unknown".
func Version() string {
	if version != "" {
		return version
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	return info.Main.Version
}
