//go:build windows

package crypto

import (
	"os"
)

// validateFileOwnership is a no-op on Windows.
// Windows uses ACLs for file permissions, not Unix-style uid/gid.
func validateFileOwnership(info os.FileInfo) error {
	// Windows doesn't support Unix-style file ownership
	// ACL-based checks would require additional Windows-specific code
	return nil
}
