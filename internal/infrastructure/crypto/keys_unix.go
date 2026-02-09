//go:build !windows

package crypto

import (
	"fmt"
	"os"
	"syscall"
)

// validateFileOwnership validates that the file is owned by the current user.
// This is only called on Unix systems.
func validateFileOwnership(info os.FileInfo) error {
	// Get file's system-specific data
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		// Can't determine ownership, skip check
		return nil
	}

	// Check if file is owned by current user
	// #nosec G115 - os.Getuid() returns non-negative value on Unix systems
	currentUID := stat.Uid
	expectedUID := os.Getuid()
	if expectedUID < 0 {
		// Getuid should never return negative on Unix, but check anyway
		return nil
	}
	if currentUID != uint32(expectedUID) {
		return fmt.Errorf("key file must be owned by current user (file uid: %d, current uid: %d)", currentUID, expectedUID)
	}

	return nil
}
