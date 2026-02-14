package interest

import (
	"os"
	"strings"
)

// Environment variable name for interest patterns.
const (
	EnvInterests      = "AGENT_COLLAB_INTERESTS"
	EnvInterestLevel  = "AGENT_COLLAB_INTEREST_LEVEL"
)

// ParsePatternsFromEnv parses comma-separated interest patterns from environment variable.
func ParsePatternsFromEnv() []string {
	return ParsePatterns(os.Getenv(EnvInterests))
}

// ParsePatterns parses comma-separated interest patterns from a string.
func ParsePatterns(value string) []string {
	if value == "" {
		return nil
	}

	parts := strings.Split(value, ",")
	patterns := make([]string, 0, len(parts))

	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			patterns = append(patterns, trimmed)
		}
	}

	return patterns
}

// ParseLevelFromEnv parses interest level from environment variable.
func ParseLevelFromEnv() InterestLevel {
	return ParseInterestLevel(os.Getenv(EnvInterestLevel))
}

// RegisterFromEnvironment creates and registers interest from environment variables.
// Returns nil if no patterns are configured.
func RegisterFromEnvironment(mgr *Manager, agentID, agentName string) (*Interest, error) {
	patterns := ParsePatternsFromEnv()
	if len(patterns) == 0 {
		return nil, nil
	}

	level := ParseLevelFromEnv()

	interest := NewInterest(agentID, agentName, patterns)
	interest.Level = level

	// Set longer TTL for environment-based interests (they should persist)
	interest.SetTTL(7 * 24 * 60 * 60 * 1e9) // 7 days

	if err := mgr.Register(interest); err != nil {
		return nil, err
	}

	return interest, nil
}

// RegisterPatterns creates and registers interest with given patterns.
func RegisterPatterns(mgr *Manager, agentID, agentName string, patterns []string, level InterestLevel) (*Interest, error) {
	if len(patterns) == 0 {
		return nil, nil
	}

	interest := NewInterest(agentID, agentName, patterns)
	interest.Level = level
	interest.SetTTL(7 * 24 * 60 * 60 * 1e9) // 7 days

	if err := mgr.Register(interest); err != nil {
		return nil, err
	}

	return interest, nil
}
