//go:build windows

package service

import (
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
)

// IsWindowsService returns true if running as a Windows service.
func IsWindowsService() bool {
	isService, _ := svc.IsWindowsService()
	return isService
}

// WriteEventLog writes to the Windows event log.
func WriteEventLog(msg string, eventType uint16) error {
	elog, err := eventlog.Open("sftp-sync")
	if err != nil {
		return err
	}
	defer elog.Close()

	switch eventType {
	case eventlog.Info:
		return elog.Info(1, msg)
	case eventlog.Warning:
		return elog.Warning(2, msg)
	case eventlog.Error:
		return elog.Error(3, msg)
	default:
		return elog.Info(1, msg)
	}
}

// InstallEventLog installs the event log source.
func InstallEventLog() error {
	return eventlog.InstallAsEventCreate("sftp-sync", eventlog.Error|eventlog.Warning|eventlog.Info)
}

// RemoveEventLog removes the event log source.
func RemoveEventLog() error {
	return eventlog.Remove("sftp-sync")
}
