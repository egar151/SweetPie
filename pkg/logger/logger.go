// Package logger provides structured logging with file rotation support.
package logger

import (
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/rs/zerolog"
	"gopkg.in/natefinch/lumberjack.v2"
)

// Config holds the logger configuration.
type Config struct {
	File       string `yaml:"file"`
	Level      string `yaml:"level"`
	MaxSizeMB  int    `yaml:"max_size_mb"`
	MaxBackups int    `yaml:"max_backups"`
	MaxAgeDays int    `yaml:"max_age_days"`
}

// Logger wraps zerolog.Logger with additional functionality.
type Logger struct {
	zerolog.Logger
	file *lumberjack.Logger
}

// New creates a new logger with the given configuration.
func New(cfg Config) (*Logger, error) {
	// Parse log level
	level, err := zerolog.ParseLevel(cfg.Level)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure time format
	zerolog.TimeFieldFormat = time.RFC3339

	var writers []io.Writer

	// Console writer (pretty format for development)
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}
	writers = append(writers, consoleWriter)

	var fileLogger *lumberjack.Logger

	// File writer with rotation
	if cfg.File != "" {
		// Ensure log directory exists
		logDir := filepath.Dir(cfg.File)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, err
		}

		fileLogger = &lumberjack.Logger{
			Filename:   cfg.File,
			MaxSize:    cfg.MaxSizeMB,
			MaxBackups: cfg.MaxBackups,
			MaxAge:     cfg.MaxAgeDays,
			Compress:   true,
		}
		writers = append(writers, fileLogger)
	}

	// Multi-writer for both console and file
	multi := zerolog.MultiLevelWriter(writers...)

	logger := zerolog.New(multi).With().Timestamp().Logger()

	return &Logger{
		Logger: logger,
		file:   fileLogger,
	}, nil
}

// Close closes the log file if one is open.
func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

// WithComponent returns a logger with a component field.
func (l *Logger) WithComponent(component string) *Logger {
	return &Logger{
		Logger: l.Logger.With().Str("component", component).Logger(),
		file:   l.file,
	}
}

// WithField returns a logger with an additional field.
func (l *Logger) WithField(key, value string) *Logger {
	return &Logger{
		Logger: l.Logger.With().Str(key, value).Logger(),
		file:   l.file,
	}
}

// Default returns a default logger that writes to stdout only.
func Default() *Logger {
	consoleWriter := zerolog.ConsoleWriter{
		Out:        os.Stdout,
		TimeFormat: time.RFC3339,
	}
	logger := zerolog.New(consoleWriter).With().Timestamp().Logger()
	return &Logger{Logger: logger}
}
