// Package rules provides file pattern matching and rule resolution.
package rules

import (
	"path/filepath"

	"github.com/gobwas/glob"

	"sftp-sync/internal/config"
)

// Matcher handles file pattern matching for transfer rules.
type Matcher struct {
	patterns map[string]glob.Glob
	rules    []config.Rule
}

// NewMatcher creates a new Matcher with the given rules.
func NewMatcher(rules []config.Rule) (*Matcher, error) {
	patterns := make(map[string]glob.Glob)

	for _, rule := range rules {
		g, err := glob.Compile(rule.Pattern)
		if err != nil {
			return nil, err
		}
		patterns[rule.Name] = g
	}

	return &Matcher{
		patterns: patterns,
		rules:    rules,
	}, nil
}

// Match finds the first matching rule for a filename.
// Returns the matched rule and true if found, or nil and false if no match.
func (m *Matcher) Match(filename string) (*config.Rule, bool) {
	// Extract just the filename without path
	name := filepath.Base(filename)

	for i, rule := range m.rules {
		pattern := m.patterns[rule.Name]
		if pattern.Match(name) {
			return &m.rules[i], true
		}
	}

	return nil, false
}

// MatchAll returns all rules that match the given filename.
func (m *Matcher) MatchAll(filename string) []*config.Rule {
	name := filepath.Base(filename)
	var matches []*config.Rule

	for i, rule := range m.rules {
		pattern := m.patterns[rule.Name]
		if pattern.Match(name) {
			matches = append(matches, &m.rules[i])
		}
	}

	return matches
}

// Rules returns all configured rules.
func (m *Matcher) Rules() []config.Rule {
	return m.rules
}

// RuleByName returns a rule by its name.
func (m *Matcher) RuleByName(name string) (*config.Rule, bool) {
	for i, rule := range m.rules {
		if rule.Name == name {
			return &m.rules[i], true
		}
	}
	return nil, false
}

// MatchesPattern checks if a filename matches a specific pattern.
func (m *Matcher) MatchesPattern(filename, pattern string) bool {
	g, err := glob.Compile(pattern)
	if err != nil {
		return false
	}
	return g.Match(filepath.Base(filename))
}

// FilterFiles filters a list of filenames, returning only those that match any rule.
func (m *Matcher) FilterFiles(filenames []string) []string {
	var matched []string
	for _, filename := range filenames {
		if _, ok := m.Match(filename); ok {
			matched = append(matched, filename)
		}
	}
	return matched
}

// FilterByDirection filters files by their transfer direction.
func (m *Matcher) FilterByDirection(filenames []string, direction string) []string {
	var matched []string
	for _, filename := range filenames {
		rule, ok := m.Match(filename)
		if !ok {
			continue
		}

		// Check if direction matches
		switch direction {
		case config.DirectionSFTPToLocal:
			if rule.Direction == config.DirectionSFTPToLocal || rule.Direction == config.DirectionBidirectional {
				matched = append(matched, filename)
			}
		case config.DirectionLocalToSFTP:
			if rule.Direction == config.DirectionLocalToSFTP || rule.Direction == config.DirectionBidirectional {
				matched = append(matched, filename)
			}
		case config.DirectionBidirectional:
			if rule.Direction == config.DirectionBidirectional {
				matched = append(matched, filename)
			}
		}
	}
	return matched
}
