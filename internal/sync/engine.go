package sync

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"sftp-sync/internal/config"
	"sftp-sync/internal/rules"
	"sftp-sync/internal/sftp"
	"sftp-sync/pkg/logger"
)

// TransferCompletedCallback is called when transfers complete successfully.
type TransferCompletedCallback func(rule, direction string, files []string)

// Engine orchestrates the file synchronization process.
type Engine struct {
	cfg                 *config.Config
	pool                *sftp.Pool
	resolver            *rules.Resolver
	state               *State
	transferer          *Transferer
	workerPool          *WorkerPool
	logger              *logger.Logger
	onTransferCompleted TransferCompletedCallback
}

// NewEngine creates a new sync Engine.
func NewEngine(cfg *config.Config, pool *sftp.Pool, log *logger.Logger) (*Engine, error) {
	resolver, err := rules.NewResolver(cfg.Rules, log)
	if err != nil {
		return nil, err
	}

	state := NewState(cfg.Global.StateFile)
	if err := state.Load(); err != nil {
		log.Warn().Err(err).Msg("Failed to load state, starting fresh")
	}

	transferer := NewTransferer(pool, state, log)
	workerPool := NewWorkerPool(4, transferer, log) // 4 concurrent workers

	return &Engine{
		cfg:        cfg,
		pool:       pool,
		resolver:   resolver,
		state:      state,
		transferer: transferer,
		workerPool: workerPool,
		logger:     log.WithComponent("engine"),
	}, nil
}

// Start starts the sync engine.
func (e *Engine) Start(ctx context.Context) {
	e.workerPool.Start(ctx)
}

// Stop stops the sync engine.
func (e *Engine) Stop() {
	e.workerPool.Stop()

	// Save state
	if err := e.state.Save(); err != nil {
		e.logger.Error().Err(err).Msg("Failed to save state")
	}
}

// SyncAll performs a full synchronization.
func (e *Engine) SyncAll(ctx context.Context) error {
	e.logger.Info().Msg("Starting full sync")

	// Sync downloads (SFTP -> Local)
	downloadResults, err := e.syncDownloads(ctx)
	if err != nil {
		e.logger.Error().Err(err).Msg("Download sync failed")
	}

	// Sync uploads (Local -> SFTP)
	uploadResults, err := e.syncUploads(ctx)
	if err != nil {
		e.logger.Error().Err(err).Msg("Upload sync failed")
	}

	// Update state
	e.state.UpdateLastPoll()
	if err := e.state.Save(); err != nil {
		e.logger.Warn().Err(err).Msg("Failed to save state")
	}

	totalSuccessful := downloadResults.Successful + uploadResults.Successful
	totalFailed := downloadResults.Failed + uploadResults.Failed

	e.logger.Info().
		Int("successful", totalSuccessful).
		Int("failed", totalFailed).
		Msg("Full sync complete")

	return nil
}

// syncDownloads syncs files from SFTP to local.
func (e *Engine) syncDownloads(ctx context.Context) (BatchResult, error) {
	result := BatchResult{}

	downloadRules := e.resolver.GetDownloadRules()
	if len(downloadRules) == 0 {
		return result, nil
	}

	e.logger.Debug().Int("rules", len(downloadRules)).Msg("Processing download rules")

	var allTasks []rules.TransferTask

	for _, rule := range downloadRules {
		client, err := e.pool.Get(ctx, rule.Connection)
		if err != nil {
			e.logger.Error().
				Err(err).
				Str("connection", rule.Connection).
				Msg("Failed to get SFTP client")
			continue
		}

		// List files in remote directory
		files, err := client.ListFiles(rule.RemotePath)
		if err != nil {
			e.logger.Warn().
				Err(err).
				Str("path", rule.RemotePath).
				Msg("Failed to list remote directory")
			continue
		}

		// Convert to filenames
		var filenames []string
		for _, f := range files {
			if !f.IsDir {
				filenames = append(filenames, f.Name)
			}
		}

		// Resolve tasks
		tasks := e.resolver.ResolveRemoteFiles(filenames)

		// Filter by state (only transfer changed files)
		for _, task := range tasks {
			// Check if file needs to be synced
			srcPath := filepath.Join(rule.RemotePath, task.FileName)
			fileInfo, _ := client.Stat(srcPath)

			if fileInfo != nil && !e.state.HasChanged(srcPath, fileInfo.Size, fileInfo.ModTime) {
				e.logger.Debug().
					Str("file", task.FileName).
					Msg("File unchanged, skipping")
				continue
			}

			allTasks = append(allTasks, task)
		}
	}

	if len(allTasks) == 0 {
		e.logger.Debug().Msg("No download tasks to process")
		return result, nil
	}

	// Process tasks
	processor := NewBatchProcessor(e.workerPool, e.logger)
	result = processor.ProcessBatch(ctx, allTasks)

	// Notify about completed transfers
	if e.onTransferCompleted != nil && len(result.SuccessfulFiles) > 0 {
		// Group by rule for notifications
		for _, rule := range downloadRules {
			e.onTransferCompleted(rule.Name, config.DirectionSFTPToLocal, result.SuccessfulFiles)
		}
	}

	return result, nil
}

// syncUploads syncs files from local to SFTP.
func (e *Engine) syncUploads(ctx context.Context) (BatchResult, error) {
	result := BatchResult{}

	uploadRules := e.resolver.GetUploadRules()
	if len(uploadRules) == 0 {
		return result, nil
	}

	e.logger.Debug().Int("rules", len(uploadRules)).Msg("Processing upload rules")

	var allTasks []rules.TransferTask

	for _, rule := range uploadRules {
		// List local files
		entries, err := os.ReadDir(rule.LocalPath)
		if err != nil {
			e.logger.Warn().
				Err(err).
				Str("path", rule.LocalPath).
				Msg("Failed to list local directory")
			continue
		}

		// Convert to filenames
		var filenames []string
		for _, entry := range entries {
			if !entry.IsDir() {
				filenames = append(filenames, entry.Name())
			}
		}

		// Resolve tasks
		tasks := e.resolver.ResolveLocalFiles(filenames)

		// Filter by state (only transfer changed files)
		for _, task := range tasks {
			localPath := filepath.Join(rule.LocalPath, task.FileName)
			info, err := os.Stat(localPath)
			if err != nil {
				continue
			}

			if !e.state.HasChanged(localPath, info.Size(), info.ModTime()) {
				e.logger.Debug().
					Str("file", task.FileName).
					Msg("File unchanged, skipping")
				continue
			}

			allTasks = append(allTasks, task)
		}
	}

	if len(allTasks) == 0 {
		e.logger.Debug().Msg("No upload tasks to process")
		return result, nil
	}

	// Process tasks
	processor := NewBatchProcessor(e.workerPool, e.logger)
	result = processor.ProcessBatch(ctx, allTasks)

	// Notify about completed transfers
	if e.onTransferCompleted != nil && len(result.SuccessfulFiles) > 0 {
		for _, rule := range uploadRules {
			e.onTransferCompleted(rule.Name, config.DirectionLocalToSFTP, result.SuccessfulFiles)
		}
	}

	return result, nil
}

// SyncFile syncs a single file based on its path and direction.
func (e *Engine) SyncFile(ctx context.Context, filename, direction string) error {
	rule, ok := e.resolver.Matcher().Match(filename)
	if !ok {
		e.logger.Debug().Str("file", filename).Msg("No matching rule for file")
		return nil
	}

	var task rules.TransferTask

	switch direction {
	case config.DirectionLocalToSFTP:
		task = rules.TransferTask{
			Rule:       rule,
			SourcePath: filepath.Join(rule.LocalPath, filename),
			DestPath:   filepath.Join(rule.RemotePath, filename),
			Direction:  direction,
			FileName:   filename,
		}
	case config.DirectionSFTPToLocal:
		task = rules.TransferTask{
			Rule:       rule,
			SourcePath: filepath.Join(rule.RemotePath, filename),
			DestPath:   filepath.Join(rule.LocalPath, filename),
			Direction:  direction,
			FileName:   filename,
		}
	default:
		return nil
	}

	result := e.transferer.TransferWithRetry(ctx, task)
	if !result.Success && result.Error != nil {
		return result.Error
	}

	return nil
}

// Poll performs a single poll cycle.
func (e *Engine) Poll(ctx context.Context) error {
	e.logger.Debug().Msg("Poll started")
	start := time.Now()

	err := e.SyncAll(ctx)

	e.logger.Debug().
		Dur("duration", time.Since(start)).
		Msg("Poll complete")

	return err
}

// State returns the engine's state tracker.
func (e *Engine) State() *State {
	return e.state
}

// Resolver returns the engine's rule resolver.
func (e *Engine) Resolver() *rules.Resolver {
	return e.resolver
}

// SetTransferCompletedCallback sets the callback for transfer completion notifications.
func (e *Engine) SetTransferCompletedCallback(cb TransferCompletedCallback) {
	e.onTransferCompleted = cb
}
