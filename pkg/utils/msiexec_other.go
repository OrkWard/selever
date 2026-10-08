//go:build !windows

package utils

import "errors"

// MSIAdminExtract is only available on Windows.
func MSIAdminExtract(msi, targetDir string) error {
	return errors.New("MSI extraction requires Windows")
}
