// Package service provides cross-platform service management.
package service

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/kardianos/service"

	"sftp-sync/internal/config"
	"sftp-sync/internal/notify"
	"sftp-sync/internal/sftp"
	"sftp-sync/internal/sync"
	"sftp-sync/internal/watcher"
	"sftp-sync/pkg/logger"
)

// Version is the service version (set at build time).
var Version = "1.0.0"

// Service wraps the SFTP sync functionality as a system service.
type Service struct {
	cfg       *config.Config
	logger    *logger.Logger
	pool      *sftp.Pool
	engine    *sync.Engine
	watcher   *watcher.Watcher
	notifier  *notify.Notifier
	ctx       context.Context
	cancel    context.CancelFunc
	svc       service.Service
}

// New creates a new Service instance.
func New(cfg *config.Config, log *logger.Logger) (*Service, error) {
	ctx, cancel := context.WithCancel(context.Background())

	s := &Service{
		cfg:    cfg,
		logger: log.WithComponent("service"),
		ctx:    ctx,
		cancel: cancel,
	}

	// Initialize notifier
	s.notifier = notify.NewNotifier(cfg.Notifications, log)

	// Initialize SFTP connection pool
	s.pool = sftp.NewPool(cfg.Connections, log)

	// Initialize sync engine
	engine, err := sync.NewEngine(cfg, s.pool, log)
	if err != nil {
		cancel()
		return nil, err
	}
	s.engine = engine

	// Set up transfer completion notifications
	engine.SetTransferCompletedCallback(func(rule, direction string, files []string) {
		s.notifier.NotifyTransferCompleted(rule, direction, files)
	})

	// Initialize file watcher if enabled
	if cfg.Global.WatchMode {
		w, err := watcher.NewWatcher(
			engine.Resolver().Matcher(),
			s.handleFileChange,
			log,
		)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to create file watcher, watch mode disabled")
		} else {
			s.watcher = w
		}
	}

	return s, nil
}

// handleFileChange is called when a watched file changes.
func (s *Service) handleFileChange(event watcher.Event) {
	s.logger.Info().
		Str("file", event.Name).
		Str("operation", event.Operation).
		Msg("File change detected, triggering sync")

	// Sync the changed file
	if err := s.engine.SyncFile(s.ctx, event.Name, config.DirectionLocalToSFTP); err != nil {
		s.logger.Error().
			Err(err).
			Str("file", event.Name).
			Msg("Failed to sync changed file")
	}
}

// Start implements service.Interface.
func (s *Service) Start(svc service.Service) error {
	s.svc = svc
	go s.run()
	return nil
}

// Stop implements service.Interface.
func (s *Service) Stop(svc service.Service) error {
	s.logger.Info().Msg("Service stop requested")
	s.cancel()
	return nil
}

// run is the main service loop.
func (s *Service) run() {
	s.logger.Info().
		Str("version", Version).
		Int("rules", len(s.cfg.Rules)).
		Int("connections", len(s.cfg.Connections)).
		Msg("Starting SFTP Sync Service")

	// Start notifier
	s.notifier.Start()
	defer s.notifier.Stop()

	// Connect to SFTP servers
	if err := s.connectWithRetry(); err != nil {
		s.logger.Error().Err(err).Msg("Failed to connect to SFTP servers")
		return
	}
	defer s.pool.Close()

	// Start sync engine
	s.engine.Start(s.ctx)
	defer s.engine.Stop()

	// Start file watcher if available
	if s.watcher != nil {
		s.startWatcher()
		defer s.watcher.Close()
	}

	// Send service start notification
	s.notifier.NotifyServiceStart(Version, len(s.cfg.Rules), len(s.cfg.Connections))

	// Perform initial sync
	s.logger.Info().Msg("Starting initial sync")
	if err := s.engine.SyncAll(s.ctx); err != nil {
		s.logger.Error().Err(err).Msg("Initial sync failed")
	}
	s.logger.Info().Msg("Initial sync complete")

	// Enter polling loop
	s.logger.Info().
		Dur("interval", s.cfg.Global.PollingIntervalDuration).
		Msg("Entering polling mode")

	ticker := time.NewTicker(s.cfg.Global.PollingIntervalDuration)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			s.logger.Info().Msg("Service shutting down")
			s.notifier.NotifyServiceStop("graceful shutdown")
			return

		case <-ticker.C:
			if err := s.engine.Poll(s.ctx); err != nil {
				s.logger.Error().Err(err).Msg("Poll failed")
			}
		}
	}
}

// connectWithRetry connects to SFTP servers with retry logic.
func (s *Service) connectWithRetry() error {
	if !s.cfg.Global.RetryOnStartup {
		return s.pool.ConnectAll(s.ctx)
	}

	for {
		err := s.pool.ConnectAll(s.ctx)
		if err == nil {
			return nil
		}

		s.logger.Warn().
			Err(err).
			Dur("retry_interval", s.cfg.Global.StartupRetryIntervalDuration).
			Msg("Failed to connect to SFTP servers, retrying...")

		// Notify about connection failure
		s.notifier.NotifyConnectionFailure("all", "multiple", err)

		select {
		case <-s.ctx.Done():
			return s.ctx.Err()
		case <-time.After(s.cfg.Global.StartupRetryIntervalDuration):
			// Continue retry loop
		}
	}
}

// startWatcher starts the file watcher for configured paths.
func (s *Service) startWatcher() {
	paths := s.engine.Resolver().GetLocalPaths()

	s.logger.Info().
		Strs("paths", paths).
		Msg("Starting file watcher")

	for _, path := range paths {
		if err := s.watcher.Watch(path); err != nil {
			s.logger.Warn().
				Err(err).
				Str("path", path).
				Msg("Failed to watch directory")
		}
	}

	s.watcher.Start(s.ctx)
}

// Run runs the service (blocking).
func (s *Service) Run() error {
	svcConfig := &service.Config{
		Name:        "sftp-sync",
		DisplayName: "SFTP Sync Service",
		Description: "Bidirectional SFTP file synchronization service",
	}

	svc, err := service.New(s, svcConfig)
	if err != nil {
		return err
	}

	return svc.Run()
}

// RunForeground runs the service in the foreground (for development).
func (s *Service) RunForeground() error {
	// Handle interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		s.logger.Info().Msg("Received shutdown signal")
		s.cancel()
	}()

	s.run()
	return nil
}

// Install installs the service.
func Install(configPath string) error {
	svcConfig := &service.Config{
		Name:        "sftp-sync",
		DisplayName: "SFTP Sync Service",
		Description: "Bidirectional SFTP file synchronization service",
		Arguments:   []string{"--config", configPath},
	}

	prg := &installProgram{}
	svc, err := service.New(prg, svcConfig)
	if err != nil {
		return err
	}

	return svc.Install()
}

// Uninstall uninstalls the service.
func Uninstall() error {
	svcConfig := &service.Config{
		Name:        "sftp-sync",
		DisplayName: "SFTP Sync Service",
		Description: "Bidirectional SFTP file synchronization service",
	}

	prg := &installProgram{}
	svc, err := service.New(prg, svcConfig)
	if err != nil {
		return err
	}

	return svc.Uninstall()
}

// Status returns the service status.
func Status() (string, error) {
	svcConfig := &service.Config{
		Name:        "sftp-sync",
		DisplayName: "SFTP Sync Service",
		Description: "Bidirectional SFTP file synchronization service",
	}

	prg := &installProgram{}
	svc, err := service.New(prg, svcConfig)
	if err != nil {
		return "", err
	}

	status, err := svc.Status()
	if err != nil {
		return "", err
	}

	switch status {
	case service.StatusRunning:
		return "running", nil
	case service.StatusStopped:
		return "stopped", nil
	default:
		return "unknown", nil
	}
}

// installProgram is a minimal program for install/uninstall operations.
type installProgram struct{}

func (p *installProgram) Start(s service.Service) error { return nil }
func (p *installProgram) Stop(s service.Service) error  { return nil }
