package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/gobwas/glob"
)

// ValidationError represents a configuration validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// Validate checks the configuration for errors.
func Validate(cfg *Config) []error {
	var errors []error

	// Validate global settings
	errors = append(errors, validateGlobal(&cfg.Global)...)

	// Validate connections
	errors = append(errors, validateConnections(cfg.Connections)...)

	// Validate rules
	errors = append(errors, validateRules(cfg.Rules, cfg.Connections)...)

	// Validate notifications
	if cfg.Notifications.Enabled {
		errors = append(errors, validateNotifications(&cfg.Notifications)...)
	}

	return errors
}

func validateGlobal(g *GlobalConfig) []error {
	var errors []error

	if g.PollingIntervalDuration <= 0 {
		errors = append(errors, ValidationError{
			Field:   "global.polling_interval",
			Message: "must be a positive duration",
		})
	}

	if g.StateFile == "" {
		errors = append(errors, ValidationError{
			Field:   "global.state_file",
			Message: "is required",
		})
	}

	if g.RetryOnStartup && g.StartupRetryIntervalDuration <= 0 {
		errors = append(errors, ValidationError{
			Field:   "global.startup_retry_interval",
			Message: "must be a positive duration when retry_on_startup is enabled",
		})
	}

	return errors
}

func validateConnections(conns map[string]SFTPConn) []error {
	var errors []error

	if len(conns) == 0 {
		errors = append(errors, ValidationError{
			Field:   "connections",
			Message: "at least one connection is required",
		})
		return errors
	}

	for name, conn := range conns {
		prefix := fmt.Sprintf("connections.%s", name)

		if conn.Host == "" {
			errors = append(errors, ValidationError{
				Field:   prefix + ".host",
				Message: "is required",
			})
		}

		if conn.Port <= 0 || conn.Port > 65535 {
			errors = append(errors, ValidationError{
				Field:   prefix + ".port",
				Message: "must be between 1 and 65535",
			})
		}

		if conn.Username == "" {
			errors = append(errors, ValidationError{
				Field:   prefix + ".username",
				Message: "is required",
			})
		}

		switch conn.AuthType {
		case AuthTypePassword:
			if conn.DecodedPassword == "" {
				errors = append(errors, ValidationError{
					Field:   prefix + ".password",
					Message: "is required for password authentication",
				})
			}
		case AuthTypeKey:
			if conn.KeyFile == "" {
				errors = append(errors, ValidationError{
					Field:   prefix + ".key_file",
					Message: "is required for key authentication",
				})
			} else if _, err := os.Stat(conn.KeyFile); os.IsNotExist(err) {
				errors = append(errors, ValidationError{
					Field:   prefix + ".key_file",
					Message: fmt.Sprintf("file does not exist: %s", conn.KeyFile),
				})
			}
		default:
			errors = append(errors, ValidationError{
				Field:   prefix + ".auth_type",
				Message: "must be 'password' or 'key'",
			})
		}

		if conn.TimeoutDuration <= 0 {
			errors = append(errors, ValidationError{
				Field:   prefix + ".timeout",
				Message: "must be a positive duration",
			})
		}
	}

	return errors
}

func validateRules(rules []Rule, conns map[string]SFTPConn) []error {
	var errors []error

	if len(rules) == 0 {
		errors = append(errors, ValidationError{
			Field:   "rules",
			Message: "at least one rule is required",
		})
		return errors
	}

	for i, rule := range rules {
		prefix := fmt.Sprintf("rules[%d]", i)

		if rule.Name == "" {
			errors = append(errors, ValidationError{
				Field:   prefix + ".name",
				Message: "is required",
			})
		}

		if rule.Pattern == "" {
			errors = append(errors, ValidationError{
				Field:   prefix + ".pattern",
				Message: "is required",
			})
		} else {
			// Validate glob pattern
			if _, err := glob.Compile(rule.Pattern); err != nil {
				errors = append(errors, ValidationError{
					Field:   prefix + ".pattern",
					Message: fmt.Sprintf("invalid glob pattern: %v", err),
				})
			}
		}

		if rule.Connection == "" {
			errors = append(errors, ValidationError{
				Field:   prefix + ".connection",
				Message: "is required",
			})
		} else if _, exists := conns[rule.Connection]; !exists {
			errors = append(errors, ValidationError{
				Field:   prefix + ".connection",
				Message: fmt.Sprintf("references undefined connection: %s", rule.Connection),
			})
		}

		switch rule.Direction {
		case DirectionSFTPToLocal, DirectionLocalToSFTP, DirectionBidirectional:
			// Valid
		default:
			errors = append(errors, ValidationError{
				Field:   prefix + ".direction",
				Message: "must be 'sftp_to_local', 'local_to_sftp', or 'bidirectional'",
			})
		}

		if rule.RemotePath == "" {
			errors = append(errors, ValidationError{
				Field:   prefix + ".remote_path",
				Message: "is required",
			})
		}

		if rule.LocalPath == "" {
			errors = append(errors, ValidationError{
				Field:   prefix + ".local_path",
				Message: "is required",
			})
		} else {
			// Ensure local path exists or can be created
			if err := os.MkdirAll(filepath.Clean(rule.LocalPath), 0755); err != nil {
				errors = append(errors, ValidationError{
					Field:   prefix + ".local_path",
					Message: fmt.Sprintf("cannot create directory: %v", err),
				})
			}
		}

		switch rule.ConflictStrategy {
		case ConflictNewerWins, ConflictRemoteWins, ConflictLocalWins, ConflictSkip, ConflictRename:
			// Valid
		case "":
			errors = append(errors, ValidationError{
				Field:   prefix + ".conflict_strategy",
				Message: "is required",
			})
		default:
			errors = append(errors, ValidationError{
				Field:   prefix + ".conflict_strategy",
				Message: "must be 'newer_wins', 'remote_wins', 'local_wins', 'skip', or 'rename'",
			})
		}

		if rule.RetryCount < 0 {
			errors = append(errors, ValidationError{
				Field:   prefix + ".retry_count",
				Message: "must be non-negative",
			})
		}

		switch rule.PostTransfer {
		case PostTransferDelete, PostTransferArchive, PostTransferKeep:
			// Valid
		case "":
			errors = append(errors, ValidationError{
				Field:   prefix + ".post_transfer",
				Message: "is required",
			})
		default:
			errors = append(errors, ValidationError{
				Field:   prefix + ".post_transfer",
				Message: "must be 'delete', 'archive', or 'keep'",
			})
		}

		if rule.PostTransfer == PostTransferArchive && rule.ArchivePath == "" {
			errors = append(errors, ValidationError{
				Field:   prefix + ".archive_path",
				Message: "is required when post_transfer is 'archive'",
			})
		}
	}

	return errors
}

func validateNotifications(n *NotificationConfig) []error {
	var errors []error

	if n.SMTPHost == "" {
		errors = append(errors, ValidationError{
			Field:   "notifications.smtp_host",
			Message: "is required when notifications are enabled",
		})
	}

	if n.SMTPPort <= 0 || n.SMTPPort > 65535 {
		errors = append(errors, ValidationError{
			Field:   "notifications.smtp_port",
			Message: "must be between 1 and 65535",
		})
	}

	if n.FromAddress == "" {
		errors = append(errors, ValidationError{
			Field:   "notifications.from_address",
			Message: "is required when notifications are enabled",
		})
	}

	if len(n.ToAddresses) == 0 {
		errors = append(errors, ValidationError{
			Field:   "notifications.to_addresses",
			Message: "at least one recipient is required when notifications are enabled",
		})
	}

	// Validate notify_on events
	validEvents := map[string]bool{
		NotifyConnectionFailure: true,
		NotifyTransferFailure:   true,
		NotifyTransferCompleted: true,
		NotifyServiceStart:      true,
		NotifyServiceStop:       true,
	}

	for _, event := range n.NotifyOn {
		if !validEvents[event] {
			errors = append(errors, ValidationError{
				Field:   "notifications.notify_on",
				Message: fmt.Sprintf("invalid event: %s", event),
			})
		}
	}

	return errors
}
