package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"sftp-sync/internal/config"
	"sftp-sync/internal/rules"
	"sftp-sync/internal/sftp"
	"sftp-sync/pkg/logger"
)

// TransferResult represents the outcome of a transfer operation.
type TransferResult struct {
	Task      rules.TransferTask
	Success   bool
	Error     error
	Duration  time.Duration
	BytesSize int64
	Checksum  string
}

// Transferer handles individual file transfers.
type Transferer struct {
	pool             *sftp.Pool
	state            *State
	conflictResolver *ConflictResolver
	logger           *logger.Logger
}

// NewTransferer creates a new Transferer.
func NewTransferer(pool *sftp.Pool, state *State, log *logger.Logger) *Transferer {
	return &Transferer{
		pool:             pool,
		state:            state,
		conflictResolver: NewConflictResolver(log),
		logger:           log.WithComponent("transfer"),
	}
}

// Transfer performs a single file transfer.
func (t *Transferer) Transfer(ctx context.Context, task rules.TransferTask) TransferResult {
	start := time.Now()
	result := TransferResult{Task: task}

	t.logger.Info().
		Str("file", task.FileName).
		Str("direction", task.Direction).
		Str("rule", task.Rule.Name).
		Msg("Starting transfer")

	// Get SFTP client
	client, err := t.pool.Get(ctx, task.Rule.Connection)
	if err != nil {
		result.Error = fmt.Errorf("failed to get SFTP connection: %w", err)
		result.Duration = time.Since(start)
		return result
	}

	// Perform transfer based on direction
	switch task.Direction {
	case config.DirectionSFTPToLocal:
		err = t.downloadFile(ctx, client, task, &result)
	case config.DirectionLocalToSFTP:
		err = t.uploadFile(ctx, client, task, &result)
	default:
		err = fmt.Errorf("unsupported direction: %s", task.Direction)
	}

	result.Duration = time.Since(start)

	if err != nil {
		result.Error = err
		t.logger.Error().
			Err(err).
			Str("file", task.FileName).
			Dur("duration", result.Duration).
			Msg("Transfer failed")
		return result
	}

	result.Success = true

	// Record successful transfer in state
	t.state.RecordTransfer(
		task.SourcePath,
		result.BytesSize,
		time.Now(),
		result.Checksum,
		task.Direction,
		task.Rule.Name,
	)

	t.logger.Info().
		Str("file", task.FileName).
		Int64("size", result.BytesSize).
		Dur("duration", result.Duration).
		Msg("Transfer complete")

	// Handle post-transfer actions
	if err := t.handlePostTransfer(ctx, client, task); err != nil {
		t.logger.Warn().
			Err(err).
			Str("file", task.FileName).
			Str("action", task.Rule.PostTransfer).
			Msg("Post-transfer action failed")
	}

	return result
}

// downloadFile downloads a file from SFTP to local.
func (t *Transferer) downloadFile(ctx context.Context, client *sftp.Client, task rules.TransferTask, result *TransferResult) error {
	// Check if destination exists for conflict resolution
	destInfo, err := os.Stat(task.DestPath)
	destExists := err == nil

	if destExists {
		// Get source info for conflict resolution
		srcInfo, err := client.Stat(task.SourcePath)
		if err != nil {
			return fmt.Errorf("failed to stat source: %w", err)
		}

		conflictInfo := ConflictInfo{
			SourcePath:    task.SourcePath,
			DestPath:      task.DestPath,
			SourceModTime: srcInfo.ModTime,
			DestModTime:   destInfo.ModTime(),
			SourceSize:    srcInfo.Size,
			DestSize:      destInfo.Size(),
			Direction:     task.Direction,
		}

		conflictResult, newDest := t.conflictResolver.Resolve(conflictInfo, task.Rule.ConflictStrategy)

		switch conflictResult {
		case ConflictResultSkip:
			return nil // Not an error, just skip
		case ConflictResultRename:
			task.DestPath = newDest
		case ConflictResultTransfer:
			// Continue with transfer
		}
	}

	// Perform download
	if err := client.Download(ctx, task.SourcePath, task.DestPath); err != nil {
		return fmt.Errorf("download failed: %w", err)
	}

	// Get file info
	info, err := os.Stat(task.DestPath)
	if err != nil {
		return fmt.Errorf("failed to stat downloaded file: %w", err)
	}
	result.BytesSize = info.Size()

	// Calculate checksum
	checksum, err := CalculateChecksum(task.DestPath)
	if err != nil {
		t.logger.Warn().Err(err).Msg("Failed to calculate checksum")
	}
	result.Checksum = checksum

	return nil
}

// uploadFile uploads a file from local to SFTP.
func (t *Transferer) uploadFile(ctx context.Context, client *sftp.Client, task rules.TransferTask, result *TransferResult) error {
	// Check if destination exists for conflict resolution
	destInfo, err := client.Stat(task.DestPath)
	destExists := err == nil

	if destExists {
		// Get source info for conflict resolution
		srcInfo, err := os.Stat(task.SourcePath)
		if err != nil {
			return fmt.Errorf("failed to stat source: %w", err)
		}

		conflictInfo := ConflictInfo{
			SourcePath:    task.SourcePath,
			DestPath:      task.DestPath,
			SourceModTime: srcInfo.ModTime(),
			DestModTime:   destInfo.ModTime,
			SourceSize:    srcInfo.Size(),
			DestSize:      destInfo.Size,
			Direction:     task.Direction,
		}

		conflictResult, newDest := t.conflictResolver.Resolve(conflictInfo, task.Rule.ConflictStrategy)

		switch conflictResult {
		case ConflictResultSkip:
			return nil // Not an error, just skip
		case ConflictResultRename:
			task.DestPath = newDest
		case ConflictResultTransfer:
			// Continue with transfer
		}
	}

	// Calculate checksum before upload
	checksum, err := CalculateChecksum(task.SourcePath)
	if err != nil {
		t.logger.Warn().Err(err).Msg("Failed to calculate checksum")
	}
	result.Checksum = checksum

	// Get source file size
	srcInfo, err := os.Stat(task.SourcePath)
	if err != nil {
		return fmt.Errorf("failed to stat source: %w", err)
	}
	result.BytesSize = srcInfo.Size()

	// Perform upload
	if err := client.Upload(ctx, task.SourcePath, task.DestPath); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	return nil
}

// handlePostTransfer handles post-transfer actions (delete, archive, keep).
func (t *Transferer) handlePostTransfer(ctx context.Context, client *sftp.Client, task rules.TransferTask) error {
	switch task.Rule.PostTransfer {
	case config.PostTransferDelete:
		return t.deleteSource(ctx, client, task)
	case config.PostTransferArchive:
		return t.archiveSource(ctx, client, task)
	case config.PostTransferKeep:
		return nil
	default:
		return nil
	}
}

// deleteSource deletes the source file after transfer.
func (t *Transferer) deleteSource(ctx context.Context, client *sftp.Client, task rules.TransferTask) error {
	switch task.Direction {
	case config.DirectionSFTPToLocal:
		// Delete from SFTP
		if err := client.Delete(task.SourcePath); err != nil {
			return fmt.Errorf("failed to delete remote file: %w", err)
		}
		t.logger.Debug().Str("file", task.SourcePath).Msg("Deleted source file from SFTP")

	case config.DirectionLocalToSFTP:
		// Delete local file
		if err := os.Remove(task.SourcePath); err != nil {
			return fmt.Errorf("failed to delete local file: %w", err)
		}
		t.logger.Debug().Str("file", task.SourcePath).Msg("Deleted source file locally")
	}

	// Remove from state
	t.state.Delete(task.SourcePath)

	return nil
}

// archiveSource moves the source file to an archive location.
func (t *Transferer) archiveSource(ctx context.Context, client *sftp.Client, task rules.TransferTask) error {
	archivePath := filepath.Join(task.Rule.ArchivePath, task.FileName)

	switch task.Direction {
	case config.DirectionSFTPToLocal:
		// Archive on SFTP
		if err := client.Rename(task.SourcePath, archivePath); err != nil {
			return fmt.Errorf("failed to archive remote file: %w", err)
		}
		t.logger.Debug().
			Str("from", task.SourcePath).
			Str("to", archivePath).
			Msg("Archived source file on SFTP")

	case config.DirectionLocalToSFTP:
		// Archive locally
		archiveDir := filepath.Dir(archivePath)
		if err := os.MkdirAll(archiveDir, 0755); err != nil {
			return fmt.Errorf("failed to create archive directory: %w", err)
		}
		if err := os.Rename(task.SourcePath, archivePath); err != nil {
			return fmt.Errorf("failed to archive local file: %w", err)
		}
		t.logger.Debug().
			Str("from", task.SourcePath).
			Str("to", archivePath).
			Msg("Archived source file locally")
	}

	// Update state with new location
	t.state.Delete(task.SourcePath)

	return nil
}

// TransferWithRetry performs a transfer with retry logic.
func (t *Transferer) TransferWithRetry(ctx context.Context, task rules.TransferTask) TransferResult {
	var result TransferResult

	for attempt := 0; attempt <= task.Rule.RetryCount; attempt++ {
		if attempt > 0 {
			t.logger.Info().
				Int("attempt", attempt+1).
				Int("max_attempts", task.Rule.RetryCount+1).
				Str("file", task.FileName).
				Msg("Retrying transfer")

			// Wait before retry
			select {
			case <-ctx.Done():
				result.Error = ctx.Err()
				return result
			case <-time.After(task.Rule.RetryDelayDuration):
			}
		}

		result = t.Transfer(ctx, task)
		if result.Success || result.Error == nil {
			return result
		}

		t.logger.Warn().
			Err(result.Error).
			Int("attempt", attempt+1).
			Str("file", task.FileName).
			Msg("Transfer attempt failed")
	}

	return result
}
