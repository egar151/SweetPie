package sftp

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
)

// FileInfo represents information about a remote file.
type FileInfo struct {
	Name    string
	Size    int64
	ModTime time.Time
	IsDir   bool
}

// ListFiles lists files in a remote directory.
func (c *Client) ListFiles(path string) ([]FileInfo, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	entries, err := c.sftpClient.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("failed to list directory %s: %w", path, err)
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		files = append(files, FileInfo{
			Name:    entry.Name(),
			Size:    entry.Size(),
			ModTime: entry.ModTime(),
			IsDir:   entry.IsDir(),
		})
	}

	return files, nil
}

// Download downloads a file from the remote server to a local path.
func (c *Client) Download(ctx context.Context, remotePath, localPath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	c.logger.Debug().
		Str("remote", remotePath).
		Str("local", localPath).
		Msg("Downloading file")

	// Open remote file
	remoteFile, err := c.sftpClient.Open(remotePath)
	if err != nil {
		return fmt.Errorf("failed to open remote file: %w", err)
	}
	defer remoteFile.Close()

	// Ensure local directory exists
	localDir := filepath.Dir(localPath)
	if err := os.MkdirAll(localDir, 0755); err != nil {
		return fmt.Errorf("failed to create local directory: %w", err)
	}

	// Create local file
	localFile, err := os.Create(localPath)
	if err != nil {
		return fmt.Errorf("failed to create local file: %w", err)
	}
	defer localFile.Close()

	// Copy with context cancellation check
	written, err := copyWithContext(ctx, localFile, remoteFile)
	if err != nil {
		os.Remove(localPath) // Clean up partial file
		return fmt.Errorf("failed to download file: %w", err)
	}

	c.logger.Debug().
		Str("remote", remotePath).
		Int64("bytes", written).
		Msg("Download complete")

	return nil
}

// Upload uploads a local file to the remote server.
func (c *Client) Upload(ctx context.Context, localPath, remotePath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	c.logger.Debug().
		Str("local", localPath).
		Str("remote", remotePath).
		Msg("Uploading file")

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Ensure remote directory exists
	remoteDir := filepath.Dir(remotePath)
	if err := c.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	// Create remote file
	remoteFile, err := c.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Copy with context cancellation check
	written, err := copyWithContext(ctx, remoteFile, localFile)
	if err != nil {
		c.sftpClient.Remove(remotePath) // Clean up partial file
		return fmt.Errorf("failed to upload file: %w", err)
	}

	c.logger.Debug().
		Str("remote", remotePath).
		Int64("bytes", written).
		Msg("Upload complete")

	return nil
}

// Delete removes a file from the remote server.
func (c *Client) Delete(remotePath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	if err := c.sftpClient.Remove(remotePath); err != nil {
		return fmt.Errorf("failed to delete %s: %w", remotePath, err)
	}

	c.logger.Debug().Str("remote", remotePath).Msg("Deleted remote file")
	return nil
}

// Rename renames/moves a file on the remote server.
func (c *Client) Rename(oldPath, newPath string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// Ensure destination directory exists
	destDir := filepath.Dir(newPath)
	if err := c.MkdirAll(destDir); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	if err := c.sftpClient.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", oldPath, newPath, err)
	}

	c.logger.Debug().
		Str("from", oldPath).
		Str("to", newPath).
		Msg("Renamed remote file")

	return nil
}

// Stat returns information about a remote file.
func (c *Client) Stat(remotePath string) (*FileInfo, error) {
	if !c.IsConnected() {
		return nil, fmt.Errorf("not connected")
	}

	info, err := c.sftpClient.Stat(remotePath)
	if err != nil {
		return nil, fmt.Errorf("failed to stat %s: %w", remotePath, err)
	}

	return &FileInfo{
		Name:    info.Name(),
		Size:    info.Size(),
		ModTime: info.ModTime(),
		IsDir:   info.IsDir(),
	}, nil
}

// Exists checks if a remote file exists.
func (c *Client) Exists(remotePath string) (bool, error) {
	if !c.IsConnected() {
		return false, fmt.Errorf("not connected")
	}

	_, err := c.sftpClient.Stat(remotePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to check existence: %w", err)
	}

	return true, nil
}

// MkdirAll creates a directory and all necessary parents on the remote server.
func (c *Client) MkdirAll(path string) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	// Check if already exists
	if info, err := c.sftpClient.Stat(path); err == nil && info.IsDir() {
		return nil
	}

	// Create parent directory first
	parent := filepath.Dir(path)
	if parent != path && parent != "/" && parent != "." {
		if err := c.MkdirAll(parent); err != nil {
			return err
		}
	}

	// Create the directory
	if err := c.sftpClient.Mkdir(path); err != nil {
		// Check if it was created by another goroutine
		if info, statErr := c.sftpClient.Stat(path); statErr == nil && info.IsDir() {
			return nil
		}
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	return nil
}

// copyWithContext copies data with context cancellation support.
func copyWithContext(ctx context.Context, dst io.Writer, src io.Reader) (int64, error) {
	buf := make([]byte, 32*1024) // 32KB buffer
	var written int64

	for {
		select {
		case <-ctx.Done():
			return written, ctx.Err()
		default:
		}

		nr, readErr := src.Read(buf)
		if nr > 0 {
			nw, writeErr := dst.Write(buf[:nr])
			if nw > 0 {
				written += int64(nw)
			}
			if writeErr != nil {
				return written, writeErr
			}
			if nr != nw {
				return written, io.ErrShortWrite
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				return written, nil
			}
			return written, readErr
		}
	}
}

// WalkDir walks a remote directory tree.
func (c *Client) WalkDir(root string, fn func(path string, info FileInfo) error) error {
	if !c.IsConnected() {
		return fmt.Errorf("not connected")
	}

	return c.walkDir(root, fn)
}

func (c *Client) walkDir(path string, fn func(path string, info FileInfo) error) error {
	entries, err := c.ListFiles(path)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		fullPath := filepath.Join(path, entry.Name)
		if err := fn(fullPath, entry); err != nil {
			if err == sftp.ErrSSHFxNoSuchFile {
				continue // Skip if file disappeared
			}
			return err
		}

		if entry.IsDir {
			if err := c.walkDir(fullPath, fn); err != nil {
				return err
			}
		}
	}

	return nil
}
