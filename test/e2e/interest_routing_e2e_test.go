package e2e

import (
	"os"
	"strings"
	"testing"

	"agent-collab/src/domain/interest"
)

// BDD E2E Test: Interest Registration from Environment Variable
// Given: AGENT_COLLAB_INTERESTS 환경변수가 설정된 경우
// When: 에이전트가 클러스터에 참여하면
// Then: Interest가 자동으로 등록되어야 함

func TestParseInterestsFromEnvironment(t *testing.T) {
	t.Run("Given AGENT_COLLAB_INTERESTS environment variable", func(t *testing.T) {
		// Simulate environment variable
		testCases := []struct {
			name     string
			envValue string
			expected []string
		}{
			{
				name:     "Single pattern",
				envValue: "auth-lib/**",
				expected: []string{"auth-lib/**"},
			},
			{
				name:     "Multiple patterns with comma",
				envValue: "user-service/**,auth-lib/token.go,auth-lib/middleware.go",
				expected: []string{"user-service/**", "auth-lib/token.go", "auth-lib/middleware.go"},
			},
			{
				name:     "Patterns with spaces",
				envValue: "auth-lib/**, user-service/api/* , api-gateway/**",
				expected: []string{"auth-lib/**", "user-service/api/*", "api-gateway/**"},
			},
			{
				name:     "Catch-all pattern",
				envValue: "**/*",
				expected: []string{"**/*"},
			},
			{
				name:     "Empty value",
				envValue: "",
				expected: []string{},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				patterns := ParseInterestPatterns(tc.envValue)

				if len(patterns) != len(tc.expected) {
					t.Fatalf("expected %d patterns, got %d: %v", len(tc.expected), len(patterns), patterns)
				}

				for i, expected := range tc.expected {
					if patterns[i] != expected {
						t.Errorf("pattern[%d]: expected %q, got %q", i, expected, patterns[i])
					}
				}
			})
		}
	})
}

func TestRegisterInterestsFromEnvironment(t *testing.T) {
	t.Run("Given agent with AGENT_COLLAB_INTERESTS set", func(t *testing.T) {
		// Set up environment
		originalEnv := os.Getenv("AGENT_COLLAB_INTERESTS")
		os.Setenv("AGENT_COLLAB_INTERESTS", "user-service/**,auth-lib/token.go")
		defer os.Setenv("AGENT_COLLAB_INTERESTS", originalEnv)

		mgr := interest.NewManager()
		agentID := "bob"
		agentName := "Bob"

		t.Run("When RegisterFromEnvironment is called", func(t *testing.T) {
			err := RegisterInterestsFromEnvironment(mgr, agentID, agentName)
			if err != nil {
				t.Fatalf("failed to register: %v", err)
			}

			t.Run("Then interest patterns are registered", func(t *testing.T) {
				interests := mgr.GetAgentInterests(agentID)
				if len(interests) != 1 {
					t.Fatalf("expected 1 interest, got %d", len(interests))
				}

				patterns := interests[0].Patterns
				if len(patterns) != 2 {
					t.Fatalf("expected 2 patterns, got %d", len(patterns))
				}

				expectedPatterns := map[string]bool{
					"user-service/**":    true,
					"auth-lib/token.go":  true,
				}

				for _, p := range patterns {
					if !expectedPatterns[p] {
						t.Errorf("unexpected pattern: %s", p)
					}
				}
			})

			t.Run("Then matching works correctly", func(t *testing.T) {
				matches := mgr.Match("user-service/main.go")
				if len(matches) == 0 {
					t.Error("expected match for user-service/main.go")
				}

				matches = mgr.Match("auth-lib/token.go")
				if len(matches) == 0 {
					t.Error("expected match for auth-lib/token.go")
				}

				matches = mgr.Match("auth-lib/jwt.go")
				hasMatch := false
				for _, m := range matches {
					if m.Interest.AgentID == agentID {
						hasMatch = true
					}
				}
				if hasMatch {
					t.Error("should NOT match auth-lib/jwt.go")
				}
			})
		})
	})
}

func TestMultiAgentInterestScenario(t *testing.T) {
	t.Run("Given multi-project test scenario", func(t *testing.T) {
		mgr := interest.NewManager()

		// Alice: auth-lib/**
		RegisterInterestsForAgent(mgr, "alice", "Alice", []string{"auth-lib/**"})

		// Bob: user-service/**, auth-lib/token.go, auth-lib/middleware.go
		RegisterInterestsForAgent(mgr, "bob", "Bob", []string{
			"user-service/**",
			"auth-lib/token.go",
			"auth-lib/middleware.go",
		})

		// Charlie: api-gateway/**, auth-lib/jwt.go, user-service/api/*
		RegisterInterestsForAgent(mgr, "charlie", "Charlie", []string{
			"api-gateway/**",
			"auth-lib/jwt.go",
			"user-service/api/*",
		})

		// Monitor: **/*
		RegisterInterestsForAgent(mgr, "monitor", "Monitor", []string{"**/*"})

		t.Run("Scenario 1: Alice modifies auth-lib/jwt.go", func(t *testing.T) {
			matches := mgr.Match("auth-lib/jwt.go")
			agents := extractMatchedAgents(matches)

			t.Run("Then Charlie receives (jwt.go in interests)", func(t *testing.T) {
				if !containsAgent(agents, "charlie") {
					t.Error("Charlie should receive auth-lib/jwt.go event")
				}
			})

			t.Run("Then Alice receives (auth-lib/** in interests)", func(t *testing.T) {
				if !containsAgent(agents, "alice") {
					t.Error("Alice should receive auth-lib/jwt.go event")
				}
			})

			t.Run("Then Monitor receives (catch-all)", func(t *testing.T) {
				if !containsAgent(agents, "monitor") {
					t.Error("Monitor should receive auth-lib/jwt.go event")
				}
			})

			t.Run("Then Bob does NOT receive (jwt.go not in interests)", func(t *testing.T) {
				if containsAgent(agents, "bob") {
					t.Error("Bob should NOT receive auth-lib/jwt.go event")
				}
			})
		})

		t.Run("Scenario 2: Alice modifies auth-lib/token.go", func(t *testing.T) {
			matches := mgr.Match("auth-lib/token.go")
			agents := extractMatchedAgents(matches)

			t.Run("Then Bob receives (token.go in interests)", func(t *testing.T) {
				if !containsAgent(agents, "bob") {
					t.Error("Bob should receive auth-lib/token.go event")
				}
			})

			t.Run("Then Alice receives (auth-lib/** in interests)", func(t *testing.T) {
				if !containsAgent(agents, "alice") {
					t.Error("Alice should receive auth-lib/token.go event")
				}
			})

			t.Run("Then Charlie does NOT receive (token.go not in interests)", func(t *testing.T) {
				if containsAgent(agents, "charlie") {
					t.Error("Charlie should NOT receive auth-lib/token.go event")
				}
			})
		})

		t.Run("Scenario 3: Bob modifies user-service/api/handler.go", func(t *testing.T) {
			matches := mgr.Match("user-service/api/handler.go")
			agents := extractMatchedAgents(matches)

			t.Run("Then Charlie receives (api/* in interests)", func(t *testing.T) {
				if !containsAgent(agents, "charlie") {
					t.Error("Charlie should receive user-service/api/handler.go event")
				}
			})

			t.Run("Then Bob receives (user-service/** in interests)", func(t *testing.T) {
				if !containsAgent(agents, "bob") {
					t.Error("Bob should receive user-service/api/handler.go event")
				}
			})

			t.Run("Then Alice does NOT receive (no user-service interest)", func(t *testing.T) {
				if containsAgent(agents, "alice") {
					t.Error("Alice should NOT receive user-service/api/handler.go event")
				}
			})
		})

		t.Run("Scenario 4: Bob modifies user-service/db/repository.go", func(t *testing.T) {
			matches := mgr.Match("user-service/db/repository.go")
			agents := extractMatchedAgents(matches)

			t.Run("Then Bob receives (user-service/** in interests)", func(t *testing.T) {
				if !containsAgent(agents, "bob") {
					t.Error("Bob should receive user-service/db/repository.go event")
				}
			})

			t.Run("Then Charlie does NOT receive (api/* only, not db/*)", func(t *testing.T) {
				if containsAgent(agents, "charlie") {
					t.Error("Charlie should NOT receive user-service/db/repository.go event")
				}
			})
		})
	})
}

// Helper functions for E2E tests

// ParseInterestPatterns parses comma-separated interest patterns.
func ParseInterestPatterns(envValue string) []string {
	if envValue == "" {
		return []string{}
	}

	parts := strings.Split(envValue, ",")
	patterns := make([]string, 0, len(parts))

	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			patterns = append(patterns, trimmed)
		}
	}

	return patterns
}

// RegisterInterestsFromEnvironment registers interests from AGENT_COLLAB_INTERESTS env var.
// Uses InterestLevelDirect to avoid proximity matching in tests.
func RegisterInterestsFromEnvironment(mgr *interest.Manager, agentID, agentName string) error {
	envValue := os.Getenv("AGENT_COLLAB_INTERESTS")
	patterns := ParseInterestPatterns(envValue)

	if len(patterns) == 0 {
		return nil
	}

	i := interest.NewInterest(agentID, agentName, patterns)
	i.Level = interest.InterestLevelDirect // Direct matching only - no proximity
	return mgr.Register(i)
}

// RegisterInterestsForAgent registers interests for an agent with given patterns.
// Uses InterestLevelDirect to avoid proximity matching in tests.
func RegisterInterestsForAgent(mgr *interest.Manager, agentID, agentName string, patterns []string) {
	i := interest.NewInterest(agentID, agentName, patterns)
	i.Level = interest.InterestLevelDirect // Direct matching only - no proximity
	_ = mgr.Register(i)
}

func extractMatchedAgents(matches []interest.InterestMatch) []string {
	seen := make(map[string]bool)
	var agents []string
	for _, m := range matches {
		if !seen[m.Interest.AgentID] {
			agents = append(agents, m.Interest.AgentID)
			seen[m.Interest.AgentID] = true
		}
	}
	return agents
}

func containsAgent(agents []string, agent string) bool {
	for _, a := range agents {
		if a == agent {
			return true
		}
	}
	return false
}
