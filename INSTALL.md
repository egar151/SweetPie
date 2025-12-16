# SFTP Sync Service - Installation Guide

## System Requirements

### Supported Operating Systems

- **Windows**: Windows 10, Windows 11, Windows Server 2016+
- **Linux**: Ubuntu 20.04+, CentOS 8+, Debian 10+
- **macOS**: 10.15+ (Catalina and later)

### Prerequisites

| Requirement | Version | Notes |
|-------------|---------|-------|
| Go | 1.21+ | Required only for building from source |
| Git | Any | Required only for cloning the repository |
| Network | - | Outbound access to SFTP servers (port 22 by default) |
| Disk Space | 50MB+ | Plus space for logs and state files |
| Memory | 64MB+ | Varies based on number of connections |

## Installation Methods

- [Method 1: Download Pre-built Binary](#method-1-download-pre-built-binary-recommended) (Recommended)
- [Method 2: Windows Complete Installation from Source](#method-2-windows-complete-installation-from-source)
- [Method 3: Linux/macOS Build from Source](#method-3-linuxmacos-build-from-source)

---

## Method 1: Download Pre-built Binary (Recommended)

1. Download the appropriate binary for your platform from the releases page
2. Extract the archive to your preferred location
3. Proceed to [Post-Installation Setup](#post-installation-setup)

---

## Method 2: Windows Complete Installation from Source

This section provides step-by-step instructions for installing on Windows from scratch.

### Step 1: Install Go

1. Download Go from https://go.dev/dl/
2. Run the installer (e.g., `go1.21.x.windows-amd64.msi`)
3. Follow the installation wizard (default options are fine)
4. Open a **new** Command Prompt or PowerShell window
5. Verify installation:

```cmd
go version
```

You should see output like: `go version go1.21.x windows/amd64`

### Step 2: Install Git (Optional, for cloning)

If you don't have Git installed:

1. Download Git from https://git-scm.com/download/win
2. Run the installer with default options
3. Verify installation:

```cmd
git --version
```

### Step 3: Get the Source Code

**Option A: Clone with Git**

```cmd
git clone <repository-url>
cd sftp-sync
```

**Option B: Download ZIP**

1. Download the source code ZIP from the repository
2. Extract to a folder (e.g., `C:\src\sftp-sync`)
3. Open Command Prompt and navigate to the folder:

```cmd
cd C:\src\sftp-sync
```

### Step 4: Download Dependencies

```cmd
go mod tidy
```

### Step 5: Build the Binary

**Command Prompt:**

```cmd
go build -o sftp-sync.exe ./cmd/sftp-sync
```

**PowerShell:**

```powershell
go build -o sftp-sync.exe ./cmd/sftp-sync
```

**Build with version information (PowerShell):**

```powershell
$commit = git rev-parse --short HEAD
$date = Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ"
go build -ldflags "-X main.version=1.0.0 -X main.commit=$commit -X main.date=$date" -o sftp-sync.exe ./cmd/sftp-sync
```

**Build with version information (Command Prompt):**

```cmd
for /f %i in ('git rev-parse --short HEAD') do set COMMIT=%i
go build -ldflags "-X main.version=1.0.0 -X main.commit=%COMMIT%" -o sftp-sync.exe ./cmd/sftp-sync
```

### Step 6: Create Directory Structure

Open Command Prompt or PowerShell **as Administrator**:

```cmd
mkdir C:\sftp-sync
mkdir C:\sftp-sync\logs
mkdir C:\data\edi
mkdir C:\data\edi\835
mkdir C:\data\edi\837
mkdir C:\data\edi\999
```

### Step 7: Copy Files

```cmd
copy sftp-sync.exe C:\sftp-sync\
copy config.yaml C:\sftp-sync\
```

### Step 8: Configure the Application

1. Open `C:\sftp-sync\config.yaml` in a text editor (Notepad, VS Code, etc.)

2. Encode your SFTP password:

```cmd
C:\sftp-sync\sftp-sync.exe --encode "your_password"
```

Output: `Encoded password: base64:eW91cl9wYXNzd29yZA==`

3. Update `config.yaml` with:
   - Your SFTP server hostname
   - Your username
   - The encoded password
   - Correct local paths (use `\\` for path separators in YAML)

Example paths in config.yaml:
```yaml
local_path: "C:\\data\\edi\\835\\"
```

### Step 9: Validate Configuration

```cmd
C:\sftp-sync\sftp-sync.exe --config C:\sftp-sync\config.yaml --validate
```

If successful, you'll see: `Configuration is valid.`

### Step 10: Test in Foreground Mode

Before installing as a service, test that everything works:

```cmd
C:\sftp-sync\sftp-sync.exe --config C:\sftp-sync\config.yaml
```

Watch for:
- Successful connection messages
- No error messages

Press `Ctrl+C` to stop.

### Step 11: Install as Windows Service

Open Command Prompt or PowerShell **as Administrator**:

```cmd
C:\sftp-sync\sftp-sync.exe --install --config C:\sftp-sync\config.yaml
```

### Step 12: Start the Service

```cmd
sc start sftp-sync
```

Or use Windows Services Manager:
1. Press `Win + R`, type `services.msc`, press Enter
2. Find "sftp-sync" in the list
3. Right-click and select "Start"

### Step 13: Verify Service is Running

```cmd
C:\sftp-sync\sftp-sync.exe --status
```

Or check in Services Manager (services.msc).

### Windows Service Management Commands

```cmd
:: Check service status
C:\sftp-sync\sftp-sync.exe --status

:: Start the service
sc start sftp-sync

:: Stop the service
sc stop sftp-sync

:: Restart the service
sc stop sftp-sync && sc start sftp-sync

:: View service configuration
sc qc sftp-sync

:: Uninstall the service
sc stop sftp-sync
C:\sftp-sync\sftp-sync.exe --uninstall
```

### Windows Firewall Configuration

Open PowerShell **as Administrator**:

```powershell
New-NetFirewallRule -DisplayName "SFTP Sync Outbound" -Direction Outbound -Protocol TCP -RemotePort 22 -Action Allow
```

### Windows Event Viewer

To view service logs in Event Viewer:
1. Press `Win + R`, type `eventvwr.msc`, press Enter
2. Navigate to: Windows Logs > Application
3. Filter by Source: "sftp-sync"

---

## Method 3: Linux/macOS Build from Source

### Step 1: Clone the Repository

```bash
git clone <repository-url>
cd sftp-sync
```

### Step 2: Download Dependencies

```bash
go mod tidy
```

### Step 3: Build the Binary

**For the current platform:**

```bash
go build -o sftp-sync ./cmd/sftp-sync
```

**Cross-compile for Windows (from Mac/Linux):**

```bash
GOOS=windows GOARCH=amd64 go build -o sftp-sync.exe ./cmd/sftp-sync
```

**Cross-compile for Linux (from Mac):**

```bash
GOOS=linux GOARCH=amd64 go build -o sftp-sync ./cmd/sftp-sync
```

**Build with version information:**

```bash
go build -ldflags "-X main.version=1.0.0 -X main.commit=$(git rev-parse --short HEAD) -X main.date=$(date -u +%Y-%m-%dT%H:%M:%SZ)" -o sftp-sync ./cmd/sftp-sync
```

---

## Post-Installation Setup

### Create Directory Structure

**Windows:**

```cmd
mkdir C:\sftp-sync
mkdir C:\sftp-sync\logs
mkdir C:\data\edi
```

**Linux/macOS:**

```bash
sudo mkdir -p /opt/sftp-sync/logs
sudo mkdir -p /var/data/edi
```

### Copy Files

**Windows:**

```cmd
copy sftp-sync.exe C:\sftp-sync\
copy config.yaml C:\sftp-sync\
```

**Linux/macOS:**

```bash
sudo cp sftp-sync /opt/sftp-sync/
sudo cp config.yaml /opt/sftp-sync/
sudo chmod +x /opt/sftp-sync/sftp-sync
```

### Configure the Application

1. Edit the configuration file (see [User Manual](USER_MANUAL.md) for details)
2. Encode your passwords:

**Windows:**
```cmd
sftp-sync.exe --encode "your_password"
```

**Linux/macOS:**
```bash
./sftp-sync --encode "your_password"
```

Output: `Encoded password: base64:eW91cl9wYXNzd29yZA==`

3. Update `config.yaml` with encoded passwords

### Validate Configuration

**Windows:**
```cmd
sftp-sync.exe --config config.yaml --validate
```

**Linux/macOS:**
```bash
./sftp-sync --config config.yaml --validate
```

If successful, you'll see: `Configuration is valid.`

---

## Linux Service Installation (systemd)

### Create Service File

Create `/etc/systemd/system/sftp-sync.service`:

```ini
[Unit]
Description=SFTP Sync Service
After=network.target

[Service]
Type=simple
User=sftp-sync
Group=sftp-sync
WorkingDirectory=/opt/sftp-sync
ExecStart=/opt/sftp-sync/sftp-sync --config /opt/sftp-sync/config.yaml
Restart=on-failure
RestartSec=10

[Install]
WantedBy=multi-user.target
```

### Create Service User

```bash
sudo useradd -r -s /bin/false sftp-sync
sudo chown -R sftp-sync:sftp-sync /opt/sftp-sync
sudo chown -R sftp-sync:sftp-sync /var/data/edi
```

### Enable and Start

```bash
sudo systemctl daemon-reload
sudo systemctl enable sftp-sync
sudo systemctl start sftp-sync
```

### Check Status

```bash
sudo systemctl status sftp-sync
sudo journalctl -u sftp-sync -f
```

---

## macOS Service Installation (launchd)

### Create Launch Agent

Create `~/Library/LaunchAgents/com.sftp-sync.plist`:

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.sftp-sync</string>
    <key>ProgramArguments</key>
    <array>
        <string>/opt/sftp-sync/sftp-sync</string>
        <string>--config</string>
        <string>/opt/sftp-sync/config.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/opt/sftp-sync/logs/stdout.log</string>
    <key>StandardErrorPath</key>
    <string>/opt/sftp-sync/logs/stderr.log</string>
</dict>
</plist>
```

### Load the Service

```bash
launchctl load ~/Library/LaunchAgents/com.sftp-sync.plist
```

### Manage the Service

```bash
# Start
launchctl start com.sftp-sync

# Stop
launchctl stop com.sftp-sync

# Unload
launchctl unload ~/Library/LaunchAgents/com.sftp-sync.plist
```

---

## Firewall Configuration

Ensure your firewall allows outbound connections on the SFTP port (default: 22).

**Windows Firewall (PowerShell as Administrator):**

```powershell
New-NetFirewallRule -DisplayName "SFTP Sync Outbound" -Direction Outbound -Protocol TCP -RemotePort 22 -Action Allow
```

**Linux (ufw):**

```bash
sudo ufw allow out 22/tcp
```

**Linux (firewalld):**

```bash
sudo firewall-cmd --permanent --add-port=22/tcp
sudo firewall-cmd --reload
```

---

## Verifying Installation

### Quick Test

Run the service in foreground mode to verify everything works:

**Windows:**
```cmd
C:\sftp-sync\sftp-sync.exe --config C:\sftp-sync\config.yaml
```

**Linux/macOS:**
```bash
./sftp-sync --config config.yaml
```

Watch the console output for:
- Successful connection to SFTP servers
- File transfers (if any files match your rules)
- No error messages

Press `Ctrl+C` to stop the foreground service.

### Check Version

**Windows:**
```cmd
sftp-sync.exe --version
```

**Linux/macOS:**
```bash
./sftp-sync --version
```

---

## Troubleshooting Installation

### "Permission denied" errors

- **Linux/macOS**: Ensure the binary is executable: `chmod +x sftp-sync`
- **Windows**: Run Command Prompt or PowerShell as Administrator for service installation

### "Configuration validation failed"

- Run `--validate` to see specific errors
- Check that all paths exist
- Verify password encoding is correct
- **Windows**: Use double backslashes in paths (`C:\\data\\edi\\`)

### Service fails to start

1. Check the log file for errors
2. Test in foreground mode first
3. Verify network connectivity to SFTP servers
4. **Windows**: Check Windows Event Viewer > Windows Logs > Application

### Cannot connect to SFTP server

1. Test connectivity:
   - **Windows**: `Test-NetConnection sftp.example.com -Port 22`
   - **Linux/macOS**: `ssh user@host -p 22`
2. Verify firewall allows outbound port 22
3. Check credentials with `--encode` to ensure proper encoding
4. For SSH keys, verify file permissions (600 on Linux/macOS)

### "go: command not found" (Windows)

- Close and reopen Command Prompt/PowerShell after installing Go
- Verify Go is in your PATH: `echo %PATH%` (cmd) or `$env:PATH` (PowerShell)
- Reinstall Go if necessary

---

## Upgrading

1. Stop the running service
2. Backup your `config.yaml` and state file
3. Replace the binary with the new version
4. Start the service
5. Check logs for any issues

**Windows:**
```cmd
sc stop sftp-sync
copy /Y sftp-sync.exe C:\sftp-sync\
sc start sftp-sync
```

**Linux:**
```bash
sudo systemctl stop sftp-sync
sudo cp sftp-sync /opt/sftp-sync/
sudo systemctl start sftp-sync
```

---

## Uninstalling

### Windows

Open Command Prompt **as Administrator**:

```cmd
sc stop sftp-sync
C:\sftp-sync\sftp-sync.exe --uninstall
rmdir /s /q C:\sftp-sync
```

### Linux

```bash
sudo systemctl stop sftp-sync
sudo systemctl disable sftp-sync
sudo rm /etc/systemd/system/sftp-sync.service
sudo systemctl daemon-reload
sudo rm -rf /opt/sftp-sync
```

### macOS

```bash
launchctl unload ~/Library/LaunchAgents/com.sftp-sync.plist
rm ~/Library/LaunchAgents/com.sftp-sync.plist
rm -rf /opt/sftp-sync
```
