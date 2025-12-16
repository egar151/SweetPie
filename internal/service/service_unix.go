//go:build !windows

package service

// IsWindowsService returns false on non-Windows platforms.
func IsWindowsService() bool {
	return false
}

// WriteEventLog is a no-op on non-Windows platforms.
func WriteEventLog(msg string, eventType uint16) error {
	return nil
}

// InstallEventLog is a no-op on non-Windows platforms.
func InstallEventLog() error {
	return nil
}

// RemoveEventLog is a no-op on non-Windows platforms.
func RemoveEventLog() error {
	return nil
}
