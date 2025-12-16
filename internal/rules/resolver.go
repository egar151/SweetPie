package rules

import (
	"path/filepath"

	"sftp-sync/internal/config"
	"sftp-sync/pkg/logger"
)

// TransferTask represents a file transfer to be performed.
type TransferTask struct {
	Rule       *config.Rule
	SourcePath string
	DestPath   string
	Direction  string
	FileName   string
}

// Resolver resolves transfer tasks based on rules.
type Resolver struct {
	matcher *Matcher
	logger  *logger.Logger
}

// NewResolver creates a new Resolver with the given rules.
func NewResolver(rules []config.Rule, log *logger.Logger) (*Resolver, error) {
	matcher, err := NewMatcher(rules)
	if err != nil {
		return nil, err
	}

	return &Resolver{
		matcher: matcher,
		logger:  log.WithComponent("resolver"),
	}, nil
}

// ResolveRemoteFiles resolves transfer tasks for files found on a remote server.
// This is used for SFTP -> Local transfers.
func (r *Resolver) ResolveRemoteFiles(files []string) []TransferTask {
	var tasks []TransferTask

	for _, filename := range files {
		rule, ok := r.matcher.Match(filename)
		if !ok {
			continue
		}

		// Check if this rule supports SFTP to Local direction
		if rule.Direction != config.DirectionSFTPToLocal && rule.Direction != config.DirectionBidirectional {
			continue
		}

		baseName := filepath.Base(filename)
		task := TransferTask{
			Rule:       rule,
			SourcePath: filepath.Join(rule.RemotePath, baseName),
			DestPath:   filepath.Join(rule.LocalPath, baseName),
			Direction:  config.DirectionSFTPToLocal,
			FileName:   baseName,
		}

		r.logger.Debug().
			Str("file", baseName).
			Str("rule", rule.Name).
			Str("direction", task.Direction).
			Msg("Resolved transfer task")

		tasks = append(tasks, task)
	}

	return tasks
}

// ResolveLocalFiles resolves transfer tasks for files found locally.
// This is used for Local -> SFTP transfers.
func (r *Resolver) ResolveLocalFiles(files []string) []TransferTask {
	var tasks []TransferTask

	for _, filename := range files {
		rule, ok := r.matcher.Match(filename)
		if !ok {
			continue
		}

		// Check if this rule supports Local to SFTP direction
		if rule.Direction != config.DirectionLocalToSFTP && rule.Direction != config.DirectionBidirectional {
			continue
		}

		baseName := filepath.Base(filename)
		task := TransferTask{
			Rule:       rule,
			SourcePath: filepath.Join(rule.LocalPath, baseName),
			DestPath:   filepath.Join(rule.RemotePath, baseName),
			Direction:  config.DirectionLocalToSFTP,
			FileName:   baseName,
		}

		r.logger.Debug().
			Str("file", baseName).
			Str("rule", rule.Name).
			Str("direction", task.Direction).
			Msg("Resolved transfer task")

		tasks = append(tasks, task)
	}

	return tasks
}

// GetRulesForConnection returns all rules that use a specific connection.
func (r *Resolver) GetRulesForConnection(connectionName string) []config.Rule {
	var rules []config.Rule
	for _, rule := range r.matcher.Rules() {
		if rule.Connection == connectionName {
			rules = append(rules, rule)
		}
	}
	return rules
}

// GetDownloadRules returns rules that involve downloading (SFTP -> Local).
func (r *Resolver) GetDownloadRules() []config.Rule {
	var rules []config.Rule
	for _, rule := range r.matcher.Rules() {
		if rule.Direction == config.DirectionSFTPToLocal || rule.Direction == config.DirectionBidirectional {
			rules = append(rules, rule)
		}
	}
	return rules
}

// GetUploadRules returns rules that involve uploading (Local -> SFTP).
func (r *Resolver) GetUploadRules() []config.Rule {
	var rules []config.Rule
	for _, rule := range r.matcher.Rules() {
		if rule.Direction == config.DirectionLocalToSFTP || rule.Direction == config.DirectionBidirectional {
			rules = append(rules, rule)
		}
	}
	return rules
}

// GetLocalPaths returns unique local paths that need to be watched.
func (r *Resolver) GetLocalPaths() []string {
	pathMap := make(map[string]bool)
	for _, rule := range r.matcher.Rules() {
		// Only watch paths for rules that upload to SFTP
		if rule.Direction == config.DirectionLocalToSFTP || rule.Direction == config.DirectionBidirectional {
			pathMap[rule.LocalPath] = true
		}
	}

	paths := make([]string, 0, len(pathMap))
	for path := range pathMap {
		paths = append(paths, path)
	}
	return paths
}

// Matcher returns the underlying Matcher.
func (r *Resolver) Matcher() *Matcher {
	return r.matcher
}
