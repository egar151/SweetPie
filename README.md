# SFTP Sync Service

A Go-based bidirectional SFTP file synchronization service that runs as a Windows service with comprehensive configuration per file pattern.

## Features

- **Bidirectional Sync**: Transfer files from SFTP to local and from local to SFTP
- **Per-File Configuration**: Each file pattern can have its own:
  - SFTP credentials (password or SSH key)
  - Transfer direction
  - Conflict resolution strategy
  - Retry policy
  - Post-transfer action (delete, archive, keep)
- **Real-time File Watching**: Optional fsnotify-based file monitoring
- **Windows Service**: Install and run as a Windows service
- **Email Notifications**: SMTP alerts for failures and service events
- **Base64 Password Encoding**: Passwords are not stored in plain text
- **Structured Logging**: JSON logs with rotation support

## Installation

### Prerequisites

- Go 1.21 or later

### Build

```bash
# Build for current platform (Mac/Linux development)
go build -o sftp-sync ./cmd/sftp-sync

# Cross-compile for Windows
GOOS=windows GOARCH=amd64 go build -o sftp-sync.exe ./cmd/sftp-sync
```

### Download Dependencies

```bash
go mod tidy
```

## Usage

### Run in Foreground (Development)

```bash
./sftp-sync --config config.yaml
```

### Validate Configuration

```bash
./sftp-sync --config config.yaml --validate
```

### Encode Password

```bash
./sftp-sync --encode "your_password"
# Output: base64:eW91cl9wYXNzd29yZA==
```

### Windows Service Commands

```bash
# Install as Windows service
sftp-sync.exe --install --config C:\sftp-sync\config.yaml

# Check service status
sftp-sync.exe --status

# Start service (using Windows sc command)
sc start sftp-sync

# Stop service
sc stop sftp-sync

# Uninstall service
sftp-sync.exe --uninstall
```

## Configuration

See `config.yaml` for a complete example. Key sections:

### Global Settings

```yaml
global:
  polling_interval: "5m"      # How often to check for files
  watch_mode: true            # Enable real-time file watching
  state_file: "./sync_state.json"
  retry_on_startup: true      # Retry if SFTP unavailable at start
```

### SFTP Connections

```yaml
connections:
  server_a:
    host: "sftp.example.com"
    port: 22
    username: "user"
    auth_type: "password"     # or "key"
    password: "base64:xxxx"   # Use --encode to generate
    timeout: "30s"
```

### Transfer Rules

```yaml
rules:
  - name: "835 Payment Files"
    pattern: "835_[0-9][0-9][0-9][0-9][0-9][0-9].*"
    connection: "server_a"
    direction: "sftp_to_local"
    remote_path: "/outbound/835/"
    local_path: "C:\\data\\edi\\835\\"
    conflict_strategy: "newer_wins"
    retry_count: 3
    retry_delay: "10s"
    post_transfer: "archive"
    archive_path: "/outbound/835/archive/"
```

### Direction Options

- `sftp_to_local`: Download files from SFTP to local
- `local_to_sftp`: Upload files from local to SFTP
- `bidirectional`: Sync in both directions

### Conflict Strategies

- `newer_wins`: Transfer if source is newer
- `remote_wins`: Always use remote version (for downloads)
- `local_wins`: Always use local version (for uploads)
- `skip`: Log conflict and skip file
- `rename`: Keep both, rename with timestamp

### Post-Transfer Actions

- `delete`: Delete source file after transfer
- `archive`: Move source to archive location
- `keep`: Leave source file in place

## Log Output

Logs are written to both console and file (if configured):

```
2025-12-16T10:30:00Z INF Starting SFTP Sync Service version=1.0.0
2025-12-16T10:30:01Z INF Connected to SFTP server connection=server_a
2025-12-16T10:30:02Z INF Transfer complete file=835_121625.edi direction=sftp_to_local
2025-12-16T10:30:03Z INF Archived source file archive=/outbound/835/archive/835_121625.edi
```

## Project Structure

```
sftp-sync/
├── cmd/sftp-sync/main.go     # Entry point
├── internal/
│   ├── config/               # Configuration parsing
│   ├── sftp/                 # SFTP client and connection pool
│   ├── sync/                 # Sync engine, transfers, state
│   ├── rules/                # Pattern matching
│   ├── watcher/              # File system monitoring
│   ├── notify/               # Email notifications
│   └── service/              # Windows service support
├── pkg/logger/               # Structured logging
├── config.yaml               # Example configuration
└── README.md
```

## Environment Variables

Passwords can reference environment variables in the config:

```yaml
password: "${SFTP_PASSWORD}"
```

The environment variable value should also be base64 encoded.

## Troubleshooting

### Service won't start

1. Check logs in the configured log file
2. Validate config: `sftp-sync.exe --validate --config config.yaml`
3. Check Windows Event Viewer for service errors

### Connection failures

1. Verify SFTP host and port are accessible
2. Check credentials (use `--encode` to verify password encoding)
3. For SSH keys, ensure key file exists and has correct permissions

### Files not syncing

1. Check if file pattern matches: patterns use glob syntax
2. Verify local/remote paths exist
3. Check state file for file tracking status
4. Enable debug logging: set `level: "debug"` in config

## License

MIT License
