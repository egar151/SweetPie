package sync

import (
	"context"
	"sync"
	"time"

	"sftp-sync/internal/rules"
	"sftp-sync/pkg/logger"
)

// WorkerPool manages concurrent transfer workers.
type WorkerPool struct {
	workers    int
	taskChan   chan rules.TransferTask
	resultChan chan TransferResult
	transferer *Transferer
	logger     *logger.Logger
	wg         sync.WaitGroup
}

// NewWorkerPool creates a new WorkerPool.
func NewWorkerPool(workers int, transferer *Transferer, log *logger.Logger) *WorkerPool {
	if workers < 1 {
		workers = 1
	}

	return &WorkerPool{
		workers:    workers,
		taskChan:   make(chan rules.TransferTask, workers*2),
		resultChan: make(chan TransferResult, workers*2),
		transferer: transferer,
		logger:     log.WithComponent("worker-pool"),
	}
}

// Start starts the worker pool.
func (p *WorkerPool) Start(ctx context.Context) {
	p.logger.Info().Int("workers", p.workers).Msg("Starting worker pool")

	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker(ctx, i)
	}
}

// worker is a single transfer worker.
func (p *WorkerPool) worker(ctx context.Context, id int) {
	defer p.wg.Done()

	p.logger.Debug().Int("worker_id", id).Msg("Worker started")

	for {
		select {
		case <-ctx.Done():
			p.logger.Debug().Int("worker_id", id).Msg("Worker stopped")
			return

		case task, ok := <-p.taskChan:
			if !ok {
				p.logger.Debug().Int("worker_id", id).Msg("Task channel closed, worker stopping")
				return
			}

			p.logger.Debug().
				Int("worker_id", id).
				Str("file", task.FileName).
				Msg("Worker processing task")

			result := p.transferer.TransferWithRetry(ctx, task)

			select {
			case p.resultChan <- result:
			case <-ctx.Done():
				return
			}
		}
	}
}

// Submit adds a task to the work queue.
func (p *WorkerPool) Submit(task rules.TransferTask) {
	p.taskChan <- task
}

// Results returns the channel for receiving transfer results.
func (p *WorkerPool) Results() <-chan TransferResult {
	return p.resultChan
}

// Stop stops the worker pool and waits for all workers to finish.
func (p *WorkerPool) Stop() {
	p.logger.Info().Msg("Stopping worker pool")
	close(p.taskChan)
	p.wg.Wait()
	close(p.resultChan)
	p.logger.Info().Msg("Worker pool stopped")
}

// Process processes a batch of tasks and collects results.
func (p *WorkerPool) Process(ctx context.Context, tasks []rules.TransferTask) []TransferResult {
	if len(tasks) == 0 {
		return nil
	}

	// Start a goroutine to submit tasks
	go func() {
		for _, task := range tasks {
			select {
			case <-ctx.Done():
				return
			case p.taskChan <- task:
			}
		}
	}()

	// Collect results
	results := make([]TransferResult, 0, len(tasks))
	for i := 0; i < len(tasks); i++ {
		select {
		case <-ctx.Done():
			return results
		case result := <-p.resultChan:
			results = append(results, result)
		}
	}

	return results
}

// BatchProcessor handles batch processing with result aggregation.
type BatchProcessor struct {
	pool   *WorkerPool
	logger *logger.Logger
}

// NewBatchProcessor creates a new BatchProcessor.
func NewBatchProcessor(pool *WorkerPool, log *logger.Logger) *BatchProcessor {
	return &BatchProcessor{
		pool:   pool,
		logger: log.WithComponent("batch-processor"),
	}
}

// ProcessBatch processes a batch of tasks and returns aggregated results.
func (p *BatchProcessor) ProcessBatch(ctx context.Context, tasks []rules.TransferTask) BatchResult {
	result := BatchResult{}

	if len(tasks) == 0 {
		return result
	}

	p.logger.Info().Int("tasks", len(tasks)).Msg("Processing batch")

	results := p.pool.Process(ctx, tasks)

	for _, r := range results {
		if r.Success {
			result.Successful++
			result.TotalBytes += r.BytesSize
		} else {
			result.Failed++
			result.Errors = append(result.Errors, TransferError{
				Task:  r.Task,
				Error: r.Error,
			})
		}
		result.TotalDuration += r.Duration
	}

	p.logger.Info().
		Int("successful", result.Successful).
		Int("failed", result.Failed).
		Int64("bytes", result.TotalBytes).
		Dur("duration", result.TotalDuration).
		Msg("Batch processing complete")

	return result
}

// BatchResult aggregates results from batch processing.
type BatchResult struct {
	Successful    int
	Failed        int
	TotalBytes    int64
	TotalDuration time.Duration
	Errors        []TransferError
}

// TransferError represents a failed transfer.
type TransferError struct {
	Task  rules.TransferTask
	Error error
}
