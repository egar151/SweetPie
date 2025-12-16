// Package sftp provides SFTP client functionality with connection pooling.
package sftp

import (
	"context"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"

	"sftp-sync/internal/config"
	"sftp-sync/pkg/logger"
)

// Client wraps an SSH and SFTP client with reconnection support.
type Client struct {
	name       string
	config     config.SFTPConn
	sshClient  *ssh.Client
	sftpClient *sftp.Client
	logger     *logger.Logger
	connected  bool
}

// NewClient creates a new SFTP client for the given connection configuration.
func NewClient(name string, cfg config.SFTPConn, log *logger.Logger) *Client {
	return &Client{
		name:   name,
		config: cfg,
		logger: log.WithComponent("sftp").WithField("connection", name),
	}
}

// Connect establishes an SSH and SFTP connection.
func (c *Client) Connect(ctx context.Context) error {
	if c.connected {
		return nil
	}

	c.logger.Info().
		Str("host", c.config.Host).
		Int("port", c.config.Port).
		Msg("Connecting to SFTP server")

	// Build SSH client config
	sshConfig, err := c.buildSSHConfig()
	if err != nil {
		return fmt.Errorf("failed to build SSH config: %w", err)
	}

	// Connect with timeout
	addr := fmt.Sprintf("%s:%d", c.config.Host, c.config.Port)

	dialer := &net.Dialer{Timeout: c.config.TimeoutDuration}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", addr, err)
	}

	// SSH handshake
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, sshConfig)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SSH handshake failed: %w", err)
	}

	c.sshClient = ssh.NewClient(sshConn, chans, reqs)

	// Start keepalive if configured
	if c.config.KeepaliveIntervalDuration > 0 {
		go c.keepalive(ctx)
	}

	// Create SFTP client
	c.sftpClient, err = sftp.NewClient(c.sshClient)
	if err != nil {
		c.sshClient.Close()
		return fmt.Errorf("failed to create SFTP client: %w", err)
	}

	c.connected = true
	c.logger.Info().Msg("Connected to SFTP server")

	return nil
}

// buildSSHConfig creates the SSH client configuration.
func (c *Client) buildSSHConfig() (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	switch c.config.AuthType {
	case config.AuthTypePassword:
		authMethods = append(authMethods, ssh.Password(c.config.DecodedPassword))

	case config.AuthTypeKey:
		keyData, err := os.ReadFile(c.config.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("failed to read key file: %w", err)
		}

		var signer ssh.Signer
		if c.config.DecodedKeyPassphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(keyData, []byte(c.config.DecodedKeyPassphrase))
		} else {
			signer, err = ssh.ParsePrivateKey(keyData)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to parse private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(signer))

	default:
		return nil, fmt.Errorf("unsupported auth type: %s", c.config.AuthType)
	}

	return &ssh.ClientConfig{
		User:            c.config.Username,
		Auth:            authMethods,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // TODO: Add proper host key verification
		Timeout:         c.config.TimeoutDuration,
	}, nil
}

// keepalive sends periodic keepalive messages to maintain the connection.
func (c *Client) keepalive(ctx context.Context) {
	ticker := time.NewTicker(c.config.KeepaliveIntervalDuration)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if c.sshClient != nil {
				_, _, err := c.sshClient.SendRequest("keepalive@openssh.com", true, nil)
				if err != nil {
					c.logger.Warn().Err(err).Msg("Keepalive failed")
				}
			}
		}
	}
}

// Close closes the SFTP and SSH connections.
func (c *Client) Close() error {
	c.connected = false

	var errs []error

	if c.sftpClient != nil {
		if err := c.sftpClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("SFTP close error: %w", err))
		}
		c.sftpClient = nil
	}

	if c.sshClient != nil {
		if err := c.sshClient.Close(); err != nil {
			errs = append(errs, fmt.Errorf("SSH close error: %w", err))
		}
		c.sshClient = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("close errors: %v", errs)
	}

	c.logger.Info().Msg("Disconnected from SFTP server")
	return nil
}

// IsConnected returns true if the client is connected.
func (c *Client) IsConnected() bool {
	return c.connected && c.sftpClient != nil
}

// SFTP returns the underlying SFTP client.
func (c *Client) SFTP() *sftp.Client {
	return c.sftpClient
}

// Name returns the connection name.
func (c *Client) Name() string {
	return c.name
}

// Reconnect closes the existing connection and establishes a new one.
func (c *Client) Reconnect(ctx context.Context) error {
	c.Close()
	return c.Connect(ctx)
}
