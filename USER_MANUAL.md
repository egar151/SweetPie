# SFTP Sync Service - User Manual

## Table of Contents

1. [Overview](#overview)
2. [Command Line Interface](#command-line-interface)
3. [Configuration Reference](#configuration-reference)
4. [Transfer Rules](#transfer-rules)
5. [Authentication](#authentication)
6. [Conflict Resolution](#conflict-resolution)
7. [Post-Transfer Actions](#post-transfer-actions)
8. [Notifications](#notifications)
9. [Logging](#logging)
10. [State Management](#state-management)
11. [Troubleshooting](#troubleshooting)
12. [Best Practices](#best-practices)

## Overview

SFTP Sync Service is a bidirectional file synchronization tool that transfers files between local directories and remote SFTP servers. It runs as a background service and supports:

- Downloading files from SFTP to local directories
- Uploading files from local directories to SFTP
- Bidirectional synchronization
- Per-file pattern configuration
- Real-time file watching
- Automatic retry on failures
- Email notifications

## Command Line Interface

### Basic Commands

```bash
# Show help
sftp-sync --help

# Show version
sftp-sync --version

# Run in foreground (development/testing)
sftp-sync --config config.yaml

# Validate configuration
sftp-sync --config config.yaml --validate

# Encode a password to base64
sftp-sync --encode "mypassword"
```

### Windows Service Commands

```bash
# Install as Windows service
sftp-sync.exe --install --config C:\path\to\config.yaml

# Check service status
sftp-sync.exe --status

# Uninstall service
sftp-sync.exe --uninstall

# Start/stop using Windows sc command
sc start sftp-sync
sc stop sftp-sync
```

### Command Line Options

| Option | Description |
|--------|-------------|
| `--config <path>` | Path to configuration file (default: `config.yaml`) |
| `--install` | Install as Windows service |
| `--uninstall` | Uninstall Windows service |
| `--status` | Show Windows service status |
| `--validate` | Validate configuration file and exit |
| `--version` | Show version information |
| `--encode <password>` | Encode a password to base64 format |

## Configuration Reference

The configuration file uses YAML format. Below is a complete reference of all settings.

### Global Settings

```yaml
global:
  polling_interval: "5m"          # How often to scan for files
  watch_mode: true                # Enable real-time file watching
  state_file: "./sync_state.json" # File to track transfer state
  retry_on_startup: true          # Retry if SFTP unavailable at startup
  startup_retry_interval: "30s"   # Interval between startup retries
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `polling_interval` | duration | `5m` | Interval between file scans |
| `watch_mode` | boolean | `true` | Enable fsnotify-based real-time monitoring |
| `state_file` | string | `./sync_state.json` | Path to state tracking file |
| `retry_on_startup` | boolean | `true` | Keep retrying if SFTP unavailable on startup |
| `startup_retry_interval` | duration | `30s` | Interval between startup connection retries |

### Duration Format

Duration values accept Go duration strings:
- `30s` - 30 seconds
- `5m` - 5 minutes
- `1h` - 1 hour
- `1h30m` - 1 hour and 30 minutes

### Logging Configuration

```yaml
logging:
  file: "./logs/sftp-sync.log"
  level: "info"
  max_size_mb: 100
  max_backups: 5
  max_age_days: 30
```

| Setting | Type | Default | Description |
|---------|------|---------|-------------|
| `file` | string | - | Log file path (logs to console if not set) |
| `level` | string | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `max_size_mb` | integer | `100` | Maximum log file size before rotation |
| `max_backups` | integer | `5` | Number of rotated log files to keep |
| `max_age_days` | integer | `30` | Maximum age of rotated log files |

### Connection Configuration

```yaml
connections:
  server_name:
    host: "sftp.example.com"
    port: 22
    username: "user"
    auth_type: "password"
    password: "base64:encoded_password"
    timeout: "30s"
    keepalive_interval: "15s"
```

| Setting | Type | Required | Description |
|---------|------|----------|-------------|
| `host` | string | Yes | SFTP server hostname or IP |
| `port` | integer | No | SFTP port (default: 22) |
| `username` | string | Yes | SFTP username |
| `auth_type` | string | Yes | `password` or `key` |
| `password` | string | Conditional | Base64 encoded password (required if auth_type is `password`) |
| `key_file` | string | Conditional | Path to SSH private key (required if auth_type is `key`) |
| `key_passphrase` | string | No | Base64 encoded key passphrase |
| `timeout` | duration | No | Connection timeout (default: 30s) |
| `keepalive_interval` | duration | No | SSH keepalive interval |

### Notifications Configuration

```yaml
notifications:
  enabled: true
  smtp_host: "smtp.example.com"
  smtp_port: 25
  from_address: "sftp-sync@example.com"
  to_addresses:
    - "admin@example.com"
  notify_on:
    - "connection_failure"
    - "transfer_failure"
    - "service_start"
    - "service_stop"
```

| Setting | Type | Description |
|---------|------|-------------|
| `enabled` | boolean | Enable/disable email notifications |
| `smtp_host` | string | SMTP server hostname |
| `smtp_port` | integer | SMTP port (25, 465, 587) |
| `from_address` | string | Sender email address |
| `to_addresses` | list | List of recipient email addresses |
| `notify_on` | list | Events to notify on |

**Notification Events:**
- `connection_failure` - SFTP connection failed
- `transfer_failure` - File transfer failed
- `service_start` - Service started
- `service_stop` - Service stopped

## Transfer Rules

Rules define how files are synchronized. They are evaluated in order, and the first matching rule wins.

### Rule Structure

```yaml
rules:
  - name: "Rule Name"
    pattern: "*.txt"
    connection: "server_name"
    direction: "sftp_to_local"
    remote_path: "/remote/path/"
    local_path: "C:\\local\\path\\"
    conflict_strategy: "newer_wins"
    retry_count: 3
    retry_delay: "10s"
    post_transfer: "archive"
    archive_path: "/archive/path/"
```

### Rule Settings

| Setting | Type | Required | Description |
|---------|------|----------|-------------|
| `name` | string | Yes | Human-readable rule name |
| `pattern` | string | Yes | Glob pattern for matching files |
| `connection` | string | Yes | Reference to connection name |
| `direction` | string | Yes | Transfer direction |
| `remote_path` | string | Yes | Path on SFTP server |
| `local_path` | string | Yes | Local filesystem path |
| `conflict_strategy` | string | No | How to handle conflicts |
| `retry_count` | integer | No | Number of retry attempts |
| `retry_delay` | duration | No | Delay between retries |
| `post_transfer` | string | No | Action after successful transfer |
| `archive_path` | string | Conditional | Archive location (required if post_transfer is `archive`) |

### Transfer Directions

| Direction | Description |
|-----------|-------------|
| `sftp_to_local` | Download files from SFTP server to local directory |
| `local_to_sftp` | Upload files from local directory to SFTP server |
| `bidirectional` | Sync files in both directions |

### Pattern Matching

Patterns use glob syntax:

| Pattern | Matches |
|---------|---------|
| `*.txt` | All `.txt` files |
| `data_*.csv` | Files starting with `data_` ending in `.csv` |
| `835_[0-9][0-9][0-9][0-9][0-9][0-9].*` | Files like `835_121625.edi` |
| `report_?????.pdf` | Files with exactly 5 characters after `report_` |

## Authentication

### Password Authentication

1. Encode your password:

```bash
sftp-sync --encode "your_password"
# Output: base64:eW91cl9wYXNzd29yZA==
```

2. Use in configuration:

```yaml
connections:
  my_server:
    auth_type: "password"
    password: "base64:eW91cl9wYXNzd29yZA=="
```

### SSH Key Authentication

```yaml
connections:
  my_server:
    auth_type: "key"
    key_file: "C:\\keys\\id_rsa"
    key_passphrase: "base64:encoded_passphrase"  # Optional
```

### Environment Variables

Passwords can reference environment variables:

```yaml
password: "${SFTP_PASSWORD}"
```

The environment variable value should also be base64 encoded.

## Conflict Resolution

When a file exists in both source and destination, the conflict strategy determines the outcome.

| Strategy | Behavior |
|----------|----------|
| `newer_wins` | Transfer file with the newer modification time |
| `remote_wins` | Always use the remote version (for downloads) |
| `local_wins` | Always use the local version (for uploads) |
| `skip` | Log the conflict and skip the file |
| `rename` | Keep both files, rename conflicting file with timestamp |

### Rename Example

If `report.txt` exists in both locations with `rename` strategy:
- Original: `report.txt`
- Renamed: `report_20251216_143022.txt`

## Post-Transfer Actions

After a successful transfer, the source file can be handled in three ways:

| Action | Behavior |
|--------|----------|
| `delete` | Permanently delete the source file |
| `archive` | Move source file to the specified archive path |
| `keep` | Leave the source file in place |

### Archive Configuration

```yaml
rules:
  - name: "Archive Example"
    post_transfer: "archive"
    archive_path: "/outbound/archive/"  # For SFTP source
    # or
    archive_path: "C:\\data\\archive\\"  # For local source
```

## Notifications

### Email Setup

The service sends emails via SMTP without authentication (common for internal relay servers):

```yaml
notifications:
  enabled: true
  smtp_host: "smtp.internal.example.com"
  smtp_port: 25
  from_address: "sftp-sync@example.com"
  to_addresses:
    - "team@example.com"
    - "oncall@example.com"
```

### Notification Events

| Event | When Triggered |
|-------|----------------|
| `connection_failure` | Cannot connect to SFTP server |
| `transfer_failure` | File transfer fails after all retries |
| `service_start` | Service starts successfully |
| `service_stop` | Service stops (gracefully or due to error) |

## Logging

### Log Levels

| Level | Description |
|-------|-------------|
| `debug` | Detailed debugging information |
| `info` | General operational messages |
| `warn` | Warning messages (non-critical issues) |
| `error` | Error messages (critical issues) |

### Log Format

Logs use structured JSON format:

```
2025-12-16T10:30:00Z INF Starting SFTP Sync Service version=1.0.0
2025-12-16T10:30:01Z INF Connected to SFTP server connection=server_a
2025-12-16T10:30:02Z INF Transfer complete file=835_121625.edi direction=sftp_to_local size=12345
2025-12-16T10:30:03Z INF Archived source file path=/outbound/835/archive/835_121625.edi
```

### Log Rotation

Logs are automatically rotated based on configuration:
- When file exceeds `max_size_mb`
- Old logs are compressed and named with timestamps
- Logs older than `max_age_days` are deleted

## State Management

### State File

The service maintains a state file (default: `sync_state.json`) that tracks:
- Files that have been transferred
- Last modification times
- Transfer timestamps

This prevents re-transferring files that haven't changed.

### Resetting State

To force re-transfer of all files:

1. Stop the service
2. Delete or rename the state file
3. Start the service

```bash
# Backup existing state
mv sync_state.json sync_state.json.backup

# Restart service to re-sync all files
```

## Troubleshooting

### Common Issues

#### Service won't start

1. Validate configuration:
   ```bash
   sftp-sync --validate --config config.yaml
   ```

2. Run in foreground to see errors:
   ```bash
   sftp-sync --config config.yaml
   ```

3. Check log file for specific errors

4. **Windows**: Check Event Viewer > Windows Logs > Application

#### Connection failures

1. Test network connectivity:
   ```bash
   telnet sftp.example.com 22
   ```

2. Verify credentials by connecting manually:
   ```bash
   sftp user@sftp.example.com
   ```

3. Check firewall allows outbound port 22

4. For SSH keys, verify permissions:
   ```bash
   chmod 600 ~/.ssh/id_rsa  # Linux/macOS
   ```

#### Files not syncing

1. **Pattern mismatch**: Test your pattern matches files
2. **Path doesn't exist**: Verify local and remote paths exist
3. **Already synced**: Check state file or delete it to re-sync
4. **Permission denied**: Verify SFTP user has read/write access

#### High memory usage

1. Reduce polling frequency
2. Disable watch_mode for large directories
3. Review number of concurrent connections

### Debug Mode

Enable debug logging for detailed information:

```yaml
logging:
  level: "debug"
```

Debug output includes:
- Connection handshakes
- File pattern matching
- Transfer progress
- State file operations

## Best Practices

### Security

1. **Never store plain-text passwords** - Always use `--encode`
2. **Use SSH keys when possible** - More secure than passwords
3. **Restrict file permissions** on config.yaml (contains credentials)
4. **Use environment variables** for passwords in automated deployments

### Performance

1. **Set appropriate polling intervals** - Don't poll too frequently
2. **Use watch_mode** for immediate transfers
3. **Configure retries wisely** - Balance between reliability and load
4. **Archive or delete** transferred files to reduce scan time

### Reliability

1. **Monitor logs** for recurring errors
2. **Configure email notifications** for critical events
3. **Test configuration** with `--validate` before deployment
4. **Use meaningful rule names** for easier troubleshooting

### Maintenance

1. **Rotate logs** - Configure appropriate retention
2. **Monitor disk space** for logs and state files
3. **Review archive directories** periodically
4. **Document your rules** - Use descriptive names and comments

### Example Production Configuration

```yaml
global:
  polling_interval: "5m"
  watch_mode: true
  state_file: "/var/lib/sftp-sync/state.json"
  retry_on_startup: true
  startup_retry_interval: "1m"

logging:
  file: "/var/log/sftp-sync/sync.log"
  level: "info"
  max_size_mb: 50
  max_backups: 10
  max_age_days: 90

notifications:
  enabled: true
  smtp_host: "smtp.internal.company.com"
  smtp_port: 25
  from_address: "sftp-sync@company.com"
  to_addresses:
    - "ops-team@company.com"
  notify_on:
    - "connection_failure"
    - "transfer_failure"

connections:
  production:
    host: "sftp.partner.com"
    port: 22
    username: "company_user"
    auth_type: "key"
    key_file: "/etc/sftp-sync/keys/partner_key"
    timeout: "60s"
    keepalive_interval: "30s"

rules:
  - name: "Inbound Partner Files"
    pattern: "PARTNER_*.dat"
    connection: "production"
    direction: "sftp_to_local"
    remote_path: "/outbound/"
    local_path: "/data/incoming/"
    conflict_strategy: "newer_wins"
    retry_count: 5
    retry_delay: "30s"
    post_transfer: "archive"
    archive_path: "/outbound/processed/"
```
