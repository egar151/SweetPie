package sync

import (
	"fmt"
	"path/filepath"
	"time"

	"sftp-sync/internal/config"
	"sftp-sync/pkg/logger"
)

// ConflictResult represents the result of conflict resolution.
type ConflictResult int

const (
	// ConflictResultTransfer indicates the file should be transferred.
	ConflictResultTransfer ConflictResult = iota
	// ConflictResultSkip indicates the file should be skipped.
	ConflictResultSkip
	// ConflictResultRename indicates the file should be renamed and both kept.
	ConflictResultRename
)

// ConflictInfo contains information about a potential conflict.
type ConflictInfo struct {
	SourcePath    string
	DestPath      string
	SourceModTime time.Time
	DestModTime   time.Time
	SourceSize    int64
	DestSize      int64
	Direction     string
}

// ConflictResolver handles conflict resolution for file transfers.
type ConflictResolver struct {
	logger *logger.Logger
}

// NewConflictResolver creates a new ConflictResolver.
func NewConflictResolver(log *logger.Logger) *ConflictResolver {
	return &ConflictResolver{
		logger: log.WithComponent("conflict"),
	}
}

// Resolve determines how to handle a conflict based on the rule's strategy.
func (r *ConflictResolver) Resolve(info ConflictInfo, strategy string) (ConflictResult, string) {
	r.logger.Debug().
		Str("source", info.SourcePath).
		Str("dest", info.DestPath).
		Str("strategy", strategy).
		Time("source_mod", info.SourceModTime).
		Time("dest_mod", info.DestModTime).
		Msg("Resolving conflict")

	switch strategy {
	case config.ConflictNewerWins:
		return r.resolveNewerWins(info)
	case config.ConflictRemoteWins:
		return r.resolveRemoteWins(info)
	case config.ConflictLocalWins:
		return r.resolveLocalWins(info)
	case config.ConflictSkip:
		return r.resolveSkip(info)
	case config.ConflictRename:
		return r.resolveRename(info)
	default:
		r.logger.Warn().
			Str("strategy", strategy).
			Msg("Unknown conflict strategy, defaulting to skip")
		return ConflictResultSkip, info.DestPath
	}
}

// resolveNewerWins transfers the file if the source is newer.
func (r *ConflictResolver) resolveNewerWins(info ConflictInfo) (ConflictResult, string) {
	if info.SourceModTime.After(info.DestModTime) {
		r.logger.Debug().
			Str("source", info.SourcePath).
			Msg("Source is newer, will transfer")
		return ConflictResultTransfer, info.DestPath
	}

	r.logger.Debug().
		Str("source", info.SourcePath).
		Msg("Destination is newer or same, skipping")
	return ConflictResultSkip, info.DestPath
}

// resolveRemoteWins always transfers if direction is SFTP -> Local.
func (r *ConflictResolver) resolveRemoteWins(info ConflictInfo) (ConflictResult, string) {
	if info.Direction == config.DirectionSFTPToLocal {
		r.logger.Debug().
			Str("source", info.SourcePath).
			Msg("Remote wins strategy, will transfer from SFTP")
		return ConflictResultTransfer, info.DestPath
	}

	r.logger.Debug().
		Str("source", info.SourcePath).
		Msg("Remote wins but direction is Local->SFTP, skipping")
	return ConflictResultSkip, info.DestPath
}

// resolveLocalWins always transfers if direction is Local -> SFTP.
func (r *ConflictResolver) resolveLocalWins(info ConflictInfo) (ConflictResult, string) {
	if info.Direction == config.DirectionLocalToSFTP {
		r.logger.Debug().
			Str("source", info.SourcePath).
			Msg("Local wins strategy, will transfer to SFTP")
		return ConflictResultTransfer, info.DestPath
	}

	r.logger.Debug().
		Str("source", info.SourcePath).
		Msg("Local wins but direction is SFTP->Local, skipping")
	return ConflictResultSkip, info.DestPath
}

// resolveSkip always skips conflicting files.
func (r *ConflictResolver) resolveSkip(info ConflictInfo) (ConflictResult, string) {
	r.logger.Info().
		Str("source", info.SourcePath).
		Str("dest", info.DestPath).
		Msg("Conflict detected, skipping per strategy")
	return ConflictResultSkip, info.DestPath
}

// resolveRename keeps both files by renaming the destination.
func (r *ConflictResolver) resolveRename(info ConflictInfo) (ConflictResult, string) {
	// Generate new destination path with timestamp
	ext := filepath.Ext(info.DestPath)
	base := info.DestPath[:len(info.DestPath)-len(ext)]
	timestamp := time.Now().Format("20060102_150405")
	newDest := fmt.Sprintf("%s_%s%s", base, timestamp, ext)

	r.logger.Info().
		Str("original", info.DestPath).
		Str("renamed", newDest).
		Msg("Renaming to avoid conflict")

	return ConflictResultRename, newDest
}

// NeedsConflictResolution checks if conflict resolution is needed.
func (r *ConflictResolver) NeedsConflictResolution(destExists bool) bool {
	return destExists
}

// GenerateRenamedPath generates a renamed path for a file to avoid conflicts.
func GenerateRenamedPath(path string) string {
	ext := filepath.Ext(path)
	base := path[:len(path)-len(ext)]
	timestamp := time.Now().Format("20060102_150405")
	return fmt.Sprintf("%s_%s%s", base, timestamp, ext)
}
