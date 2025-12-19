// Package config defines the configuration structures for the SFTP sync service.
package config

import (
	"time"

	"sftp-sync/pkg/logger"
)

// Config is the root configuration structure.
type Config struct {
	Global        GlobalConfig        `yaml:"global"`
	Logging       logger.Config       `yaml:"logging"`
	Notifications NotificationConfig  `yaml:"notifications"`
	Connections   map[string]SFTPConn `yaml:"connections"`
	Rules         []Rule              `yaml:"rules"`
}

// GlobalConfig holds global service settings.
type GlobalConfig struct {
	PollingInterval      string `yaml:"polling_interval"`
	WatchMode            bool   `yaml:"watch_mode"`
	StateFile            string `yaml:"state_file"`
	RetryOnStartup       bool   `yaml:"retry_on_startup"`
	StartupRetryInterval string `yaml:"startup_retry_interval"`

	// Parsed durations (populated after loading)
	PollingIntervalDuration      time.Duration `yaml:"-"`
	StartupRetryIntervalDuration time.Duration `yaml:"-"`
}

// NotificationConfig holds email notification settings.
type NotificationConfig struct {
	Enabled       bool     `yaml:"enabled"`
	SMTPHost      string   `yaml:"smtp_host"`
	SMTPPort      int      `yaml:"smtp_port"`
	SkipTLSVerify bool     `yaml:"skip_tls_verify"`
	FromAddress   string   `yaml:"from_address"`
	ToAddresses   []string `yaml:"to_addresses"`
	NotifyOn      []string `yaml:"notify_on"`
}

// SFTPConn represents an SFTP connection configuration.
type SFTPConn struct {
	Host              string `yaml:"host"`
	Port              int    `yaml:"port"`
	Username          string `yaml:"username"`
	AuthType          string `yaml:"auth_type"` // "password" or "key"
	Password          string `yaml:"password"`
	KeyFile           string `yaml:"key_file"`
	KeyPassphrase     string `yaml:"key_passphrase"`
	Timeout           string `yaml:"timeout"`
	KeepaliveInterval string `yaml:"keepalive_interval"`

	// Parsed durations (populated after loading)
	TimeoutDuration           time.Duration `yaml:"-"`
	KeepaliveIntervalDuration time.Duration `yaml:"-"`

	// Decoded password (populated after loading)
	DecodedPassword      string `yaml:"-"`
	DecodedKeyPassphrase string `yaml:"-"`
}

// Rule defines a file transfer rule.
type Rule struct {
	Name             string `yaml:"name"`
	Pattern          string `yaml:"pattern"`
	Connection       string `yaml:"connection"`
	Direction        string `yaml:"direction"` // "sftp_to_local", "local_to_sftp", "bidirectional"
	RemotePath       string `yaml:"remote_path"`
	LocalPath        string `yaml:"local_path"`
	ConflictStrategy string `yaml:"conflict_strategy"` // "newer_wins", "remote_wins", "local_wins", "skip", "rename"
	RetryCount       int    `yaml:"retry_count"`
	RetryDelay       string `yaml:"retry_delay"`
	PostTransfer     string `yaml:"post_transfer"` // "delete", "archive", "keep"
	ArchivePath      string `yaml:"archive_path"`

	// Parsed duration (populated after loading)
	RetryDelayDuration time.Duration `yaml:"-"`
}

// Direction constants
const (
	DirectionSFTPToLocal  = "sftp_to_local"
	DirectionLocalToSFTP  = "local_to_sftp"
	DirectionBidirectional = "bidirectional"
)

// ConflictStrategy constants
const (
	ConflictNewerWins  = "newer_wins"
	ConflictRemoteWins = "remote_wins"
	ConflictLocalWins  = "local_wins"
	ConflictSkip       = "skip"
	ConflictRename     = "rename"
)

// PostTransfer constants
const (
	PostTransferDelete  = "delete"
	PostTransferArchive = "archive"
	PostTransferKeep    = "keep"
)

// AuthType constants
const (
	AuthTypePassword = "password"
	AuthTypeKey      = "key"
)

// NotifyEvent constants
const (
	NotifyConnectionFailure  = "connection_failure"
	NotifyTransferFailure    = "transfer_failure"
	NotifyTransferCompleted  = "transfer_completed"
	NotifyServiceStart       = "service_start"
	NotifyServiceStop        = "service_stop"
)
