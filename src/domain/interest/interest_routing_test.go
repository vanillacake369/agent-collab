package interest_test

import (
	"testing"

	"agent-collab/src/domain/interest"
)

// BDD Test: Interest Registration and Pattern Matching
// Given: 에이전트가 특정 파일 패턴에 관심을 등록한 경우
// When: 해당 패턴과 일치하는 파일이 변경되면
// Then: 해당 에이전트에게 이벤트가 전달되어야 함

func TestInterestManager_RegisterAndMatch(t *testing.T) {
	t.Run("Given agent registers interest with glob pattern", func(t *testing.T) {
		mgr := interest.NewManager()

		// Alice는 auth-lib/** 패턴에 관심 등록
		aliceInterest := interest.NewInterest("alice", "Alice", []string{"auth-lib/**"})
		err := mgr.Register(aliceInterest)
		if err != nil {
			t.Fatalf("failed to register interest: %v", err)
		}

		t.Run("When matching file path auth-lib/jwt.go", func(t *testing.T) {
			matches := mgr.Match("auth-lib/jwt.go")

			t.Run("Then Alice should be matched", func(t *testing.T) {
				if len(matches) != 1 {
					t.Fatalf("expected 1 match, got %d", len(matches))
				}
				if matches[0].Interest.AgentID != "alice" {
					t.Errorf("expected alice, got %s", matches[0].Interest.AgentID)
				}
				if matches[0].MatchType != interest.MatchTypeDirect {
					t.Errorf("expected direct match, got %s", matches[0].MatchType)
				}
			})
		})

		t.Run("When matching nested path auth-lib/internal/crypto.go", func(t *testing.T) {
			matches := mgr.Match("auth-lib/internal/crypto.go")

			t.Run("Then Alice should be matched (recursive glob)", func(t *testing.T) {
				if len(matches) != 1 {
					t.Fatalf("expected 1 match for recursive glob, got %d", len(matches))
				}
			})
		})

		t.Run("When matching unrelated path user-service/main.go", func(t *testing.T) {
			matches := mgr.Match("user-service/main.go")

			t.Run("Then Alice should NOT be matched", func(t *testing.T) {
				for _, m := range matches {
					if m.Interest.AgentID == "alice" {
						t.Errorf("Alice should not match user-service/main.go")
					}
				}
			})
		})
	})
}

func TestInterestManager_MultipleAgentsWithOverlappingPatterns(t *testing.T) {
	t.Run("Given multiple agents with overlapping interests", func(t *testing.T) {
		mgr := interest.NewManager()

		// Alice: auth-lib/**
		aliceInterest := interest.NewInterest("alice", "Alice", []string{"auth-lib/**"})
		aliceInterest.Level = interest.InterestLevelDirect // Direct matching only
		_ = mgr.Register(aliceInterest)

		// Bob: user-service/**, auth-lib/token.go
		bobInterest := interest.NewInterest("bob", "Bob", []string{
			"user-service/**",
			"auth-lib/token.go",
		})
		bobInterest.Level = interest.InterestLevelDirect // Direct matching only
		_ = mgr.Register(bobInterest)

		// Charlie: api-gateway/**, auth-lib/jwt.go, user-service/api/*
		charlieInterest := interest.NewInterest("charlie", "Charlie", []string{
			"api-gateway/**",
			"auth-lib/jwt.go",
			"user-service/api/*",
		})
		charlieInterest.Level = interest.InterestLevelDirect // Direct matching only
		_ = mgr.Register(charlieInterest)

		t.Run("When auth-lib/jwt.go changes", func(t *testing.T) {
			matches := mgr.Match("auth-lib/jwt.go")
			matchedAgents := extractAgentIDs(matches)

			t.Run("Then Alice and Charlie should be matched, Bob should NOT", func(t *testing.T) {
				assertContains(t, matchedAgents, "alice", "Alice should match auth-lib/**")
				assertContains(t, matchedAgents, "charlie", "Charlie should match auth-lib/jwt.go")
				assertNotContains(t, matchedAgents, "bob", "Bob should NOT match auth-lib/jwt.go")
			})
		})

		t.Run("When auth-lib/token.go changes", func(t *testing.T) {
			matches := mgr.Match("auth-lib/token.go")
			matchedAgents := extractAgentIDs(matches)

			t.Run("Then Alice and Bob should be matched, Charlie should NOT", func(t *testing.T) {
				assertContains(t, matchedAgents, "alice", "Alice should match auth-lib/**")
				assertContains(t, matchedAgents, "bob", "Bob should match auth-lib/token.go")
				assertNotContains(t, matchedAgents, "charlie", "Charlie should NOT match auth-lib/token.go")
			})
		})

		t.Run("When user-service/api/handler.go changes", func(t *testing.T) {
			matches := mgr.Match("user-service/api/handler.go")
			matchedAgents := extractAgentIDs(matches)

			t.Run("Then Bob and Charlie should be matched, Alice should NOT", func(t *testing.T) {
				assertContains(t, matchedAgents, "bob", "Bob should match user-service/**")
				assertContains(t, matchedAgents, "charlie", "Charlie should match user-service/api/*")
				assertNotContains(t, matchedAgents, "alice", "Alice should NOT match user-service/api/handler.go")
			})
		})

		t.Run("When user-service/db/repository.go changes", func(t *testing.T) {
			matches := mgr.Match("user-service/db/repository.go")
			matchedAgents := extractAgentIDs(matches)

			t.Run("Then Bob should be matched, Charlie should NOT (api/* only)", func(t *testing.T) {
				assertContains(t, matchedAgents, "bob", "Bob should match user-service/**")
				assertNotContains(t, matchedAgents, "charlie", "Charlie should NOT match user-service/db/*")
				assertNotContains(t, matchedAgents, "alice", "Alice should NOT match user-service/db/*")
			})
		})
	})
}

func TestInterestManager_WildcardPatterns(t *testing.T) {
	t.Run("Given Monitor agent with catch-all pattern", func(t *testing.T) {
		mgr := interest.NewManager()

		// Monitor: **/* (모든 파일)
		monitorInterest := interest.NewInterest("monitor", "Monitor", []string{"**/*"})
		_ = mgr.Register(monitorInterest)

		t.Run("When any file changes", func(t *testing.T) {
			testPaths := []string{
				"auth-lib/jwt.go",
				"user-service/main.go",
				"api-gateway/router.go",
				"deeply/nested/path/file.go",
			}

			for _, path := range testPaths {
				matches := mgr.Match(path)
				t.Run("Then Monitor should match "+path, func(t *testing.T) {
					matchedAgents := extractAgentIDs(matches)
					assertContains(t, matchedAgents, "monitor", "Monitor should match "+path)
				})
			}
		})
	})
}

func TestInterestManager_SpecificFilePattern(t *testing.T) {
	t.Run("Given agent interested in specific files", func(t *testing.T) {
		mgr := interest.NewManager()

		// Bob은 특정 파일들에만 관심 (Direct matching only)
		bobInterest := interest.NewInterest("bob", "Bob", []string{
			"auth-lib/token.go",
			"auth-lib/middleware.go",
		})
		bobInterest.Level = interest.InterestLevelDirect // No proximity matching
		_ = mgr.Register(bobInterest)

		t.Run("When auth-lib/token.go changes", func(t *testing.T) {
			matches := mgr.Match("auth-lib/token.go")

			t.Run("Then Bob should be matched", func(t *testing.T) {
				if len(matches) != 1 {
					t.Fatalf("expected 1 match, got %d", len(matches))
				}
			})
		})

		t.Run("When auth-lib/jwt.go changes (not in interest)", func(t *testing.T) {
			matches := mgr.Match("auth-lib/jwt.go")

			t.Run("Then Bob should NOT be matched", func(t *testing.T) {
				for _, m := range matches {
					if m.Interest.AgentID == "bob" {
						t.Errorf("Bob should not match auth-lib/jwt.go")
					}
				}
			})
		})
	})
}

func TestInterestManager_InterestLevelFiltering(t *testing.T) {
	t.Run("Given agent with InterestLevelLocksOnly", func(t *testing.T) {
		mgr := interest.NewManager()

		aliceInterest := interest.NewInterest("alice", "Alice", []string{"auth-lib/**"})
		aliceInterest.Level = interest.InterestLevelLocksOnly
		_ = mgr.Register(aliceInterest)

		t.Run("When file in pattern changes", func(t *testing.T) {
			matches := mgr.Match("auth-lib/jwt.go")

			t.Run("Then match should indicate locks only interest", func(t *testing.T) {
				if len(matches) != 1 {
					t.Fatalf("expected 1 match, got %d", len(matches))
				}
				if matches[0].Interest.Level != interest.InterestLevelLocksOnly {
					t.Errorf("expected locks_only level")
				}
			})
		})
	})
}

func TestInterestManager_ExpiredInterests(t *testing.T) {
	t.Run("Given expired interest", func(t *testing.T) {
		mgr := interest.NewManager()

		aliceInterest := interest.NewInterest("alice", "Alice", []string{"auth-lib/**"})
		// 만료 시간을 과거로 설정
		aliceInterest.SetTTL(-1)
		_ = mgr.Register(aliceInterest)

		t.Run("When matching file", func(t *testing.T) {
			matches := mgr.Match("auth-lib/jwt.go")

			t.Run("Then expired interest should NOT be matched", func(t *testing.T) {
				for _, m := range matches {
					if m.Interest.AgentID == "alice" {
						t.Errorf("expired interest should not match")
					}
				}
			})
		})
	})
}

// Helper functions

func extractAgentIDs(matches []interest.InterestMatch) []string {
	var ids []string
	seen := make(map[string]bool)
	for _, m := range matches {
		if !seen[m.Interest.AgentID] {
			ids = append(ids, m.Interest.AgentID)
			seen[m.Interest.AgentID] = true
		}
	}
	return ids
}

func assertContains(t *testing.T, slice []string, value, msg string) {
	t.Helper()
	for _, v := range slice {
		if v == value {
			return
		}
	}
	t.Errorf("%s: %v does not contain %s", msg, slice, value)
}

func assertNotContains(t *testing.T, slice []string, value, msg string) {
	t.Helper()
	for _, v := range slice {
		if v == value {
			t.Errorf("%s: %v should not contain %s", msg, slice, value)
			return
		}
	}
}
