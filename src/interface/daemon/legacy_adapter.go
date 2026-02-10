// Package daemon provides HTTP server functionality for the agent-collab daemon.
// This file provides legacy API adapters for backwards compatibility.
package daemon

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	v1 "agent-collab/src/api/v1"
)

// LegacyAdapter provides backwards compatibility for the old API endpoints.
type LegacyAdapter struct {
	// apiServerURL is the URL of the new API server
	apiServerURL string
	client       *http.Client
}

// NewLegacyAdapter creates a new legacy adapter.
func NewLegacyAdapter(apiServerURL string) *LegacyAdapter {
	return &LegacyAdapter{
		apiServerURL: apiServerURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RegisterRoutes registers legacy routes on the given mux.
func (a *LegacyAdapter) RegisterRoutes(mux *http.ServeMux) {
	// Lock endpoints
	mux.HandleFunc("POST /lock/acquire", a.acquireLock)
	mux.HandleFunc("POST /lock/release", a.releaseLock)
	mux.HandleFunc("GET /lock/list", a.listLocks)

	// Context endpoints
	mux.HandleFunc("POST /context/share", a.shareContext)
	mux.HandleFunc("POST /context/search", a.searchContext)

	// Agent endpoints
	mux.HandleFunc("GET /agents/list", a.listAgents)

	// Status endpoints
	mux.HandleFunc("GET /status", a.getStatus)
}

// Legacy request/response types for backwards compatibility

// LegacyLockRequest is the old lock acquire request format.
type LegacyLockRequest struct {
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	Intention string `json:"intention"`
	TTL       int    `json:"ttl,omitempty"` // seconds
}

// LegacyLockResponse is the old lock response format.
type LegacyLockResponse struct {
	Success      bool   `json:"success"`
	LockID       string `json:"lock_id,omitempty"`
	Message      string `json:"message,omitempty"`
	FencingToken uint64 `json:"fencing_token,omitempty"`
	ExpiresAt    string `json:"expires_at,omitempty"`
}

// LegacyReleaseLockRequest is the old lock release request format.
type LegacyReleaseLockRequest struct {
	LockID string `json:"lock_id"`
}

// LegacyContextShareRequest is the old context share request format.
type LegacyContextShareRequest struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

// LegacyContextShareResponse is the old context share response format.
type LegacyContextShareResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// LegacyContextSearchRequest is the old context search request format.
type LegacyContextSearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit,omitempty"`
}

// LegacyContextSearchResult is the old context search result format.
type LegacyContextSearchResult struct {
	FilePath string  `json:"file_path"`
	Content  string  `json:"content"`
	Score    float32 `json:"score"`
}

// LegacyContextSearchResponse is the old context search response format.
type LegacyContextSearchResponse struct {
	Success bool                        `json:"success"`
	Results []LegacyContextSearchResult `json:"results,omitempty"`
	Message string                      `json:"message,omitempty"`
}

// LegacyAgentInfo is the old agent info format.
type LegacyAgentInfo struct {
	ID           string   `json:"id"`
	Name         string   `json:"name,omitempty"`
	Provider     string   `json:"provider"`
	Capabilities []string `json:"capabilities,omitempty"`
	Status       string   `json:"status"`
}

// LegacyAgentsResponse is the old agents list response format.
type LegacyAgentsResponse struct {
	Success bool              `json:"success"`
	Agents  []LegacyAgentInfo `json:"agents,omitempty"`
	Message string            `json:"message,omitempty"`
}

// LegacyLockInfo is the old lock info format.
type LegacyLockInfo struct {
	LockID    string `json:"lock_id"`
	FilePath  string `json:"file_path"`
	StartLine int    `json:"start_line,omitempty"`
	EndLine   int    `json:"end_line,omitempty"`
	HolderID  string `json:"holder_id"`
	Intention string `json:"intention"`
	ExpiresAt string `json:"expires_at"`
}

// LegacyLocksResponse is the old locks list response format.
type LegacyLocksResponse struct {
	Success bool             `json:"success"`
	Locks   []LegacyLockInfo `json:"locks,omitempty"`
	Message string           `json:"message,omitempty"`
}

// Handlers

func (a *LegacyAdapter) acquireLock(w http.ResponseWriter, r *http.Request) {
	var req LegacyLockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeJSON(w, http.StatusBadRequest, LegacyLockResponse{
			Success: false,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// Convert to new format
	ttl := time.Duration(req.TTL) * time.Second
	if ttl == 0 {
		ttl = 5 * time.Minute
	}

	lockType := v1.LockTargetTypeFile
	if req.StartLine > 0 {
		lockType = v1.LockTargetTypeLineRange
	}

	lock := v1.Lock{
		TypeMeta: v1.TypeMeta{
			Kind:       v1.LockKind,
			APIVersion: v1.GroupVersion,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:              generateLockName(req.FilePath, req.StartLine, req.EndLine),
			CreationTimestamp: time.Now(),
		},
		Spec: v1.LockSpec{
			Target: v1.LockTarget{
				Type:      lockType,
				FilePath:  req.FilePath,
				StartLine: int32(req.StartLine),
				EndLine:   int32(req.EndLine),
			},
			Intention: req.Intention,
			TTL:       v1.Duration{Duration: ttl},
			Exclusive: true,
		},
	}

	// Call new API
	body, _ := json.Marshal(lock)
	resp, err := a.client.Post(a.apiServerURL+"/api/v1/locks", "application/json", bytes.NewReader(body))
	if err != nil {
		a.writeJSON(w, http.StatusInternalServerError, LegacyLockResponse{
			Success: false,
			Message: "failed to acquire lock: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		respBody, _ := io.ReadAll(resp.Body)
		a.writeJSON(w, resp.StatusCode, LegacyLockResponse{
			Success: false,
			Message: string(respBody),
		})
		return
	}

	var created v1.Lock
	json.NewDecoder(resp.Body).Decode(&created)

	expiresAt := ""
	if created.Status.ExpiresAt != nil {
		expiresAt = created.Status.ExpiresAt.Format(time.RFC3339)
	}

	a.writeJSON(w, http.StatusOK, LegacyLockResponse{
		Success:      true,
		LockID:       created.Name,
		FencingToken: created.Status.FencingToken,
		ExpiresAt:    expiresAt,
	})
}

func (a *LegacyAdapter) releaseLock(w http.ResponseWriter, r *http.Request) {
	var req LegacyReleaseLockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeJSON(w, http.StatusBadRequest, LegacyLockResponse{
			Success: false,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// Call new API
	delReq, _ := http.NewRequest("DELETE", a.apiServerURL+"/api/v1/locks/"+req.LockID, nil)
	resp, err := a.client.Do(delReq)
	if err != nil {
		a.writeJSON(w, http.StatusInternalServerError, LegacyLockResponse{
			Success: false,
			Message: "failed to release lock: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		a.writeJSON(w, resp.StatusCode, LegacyLockResponse{
			Success: false,
			Message: string(respBody),
		})
		return
	}

	a.writeJSON(w, http.StatusOK, LegacyLockResponse{
		Success: true,
		Message: "lock released",
	})
}

func (a *LegacyAdapter) listLocks(w http.ResponseWriter, r *http.Request) {
	resp, err := a.client.Get(a.apiServerURL + "/api/v1/locks")
	if err != nil {
		a.writeJSON(w, http.StatusInternalServerError, LegacyLocksResponse{
			Success: false,
			Message: "failed to list locks: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	var list v1.LockList
	json.NewDecoder(resp.Body).Decode(&list)

	locks := make([]LegacyLockInfo, 0, len(list.Items))
	for _, lock := range list.Items {
		expiresAt := ""
		if lock.Status.ExpiresAt != nil {
			expiresAt = lock.Status.ExpiresAt.Format(time.RFC3339)
		}
		locks = append(locks, LegacyLockInfo{
			LockID:    lock.Name,
			FilePath:  lock.Spec.Target.FilePath,
			StartLine: int(lock.Spec.Target.StartLine),
			EndLine:   int(lock.Spec.Target.EndLine),
			HolderID:  lock.Spec.HolderID,
			Intention: lock.Spec.Intention,
			ExpiresAt: expiresAt,
		})
	}

	a.writeJSON(w, http.StatusOK, LegacyLocksResponse{
		Success: true,
		Locks:   locks,
	})
}

func (a *LegacyAdapter) shareContext(w http.ResponseWriter, r *http.Request) {
	var req LegacyContextShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeJSON(w, http.StatusBadRequest, LegacyContextShareResponse{
			Success: false,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// Convert to new format
	ctx := v1.Context{
		TypeMeta: v1.TypeMeta{
			Kind:       v1.ContextKind,
			APIVersion: v1.GroupVersion,
		},
		ObjectMeta: v1.ObjectMeta{
			Name:              generateContextName(req.FilePath),
			CreationTimestamp: time.Now(),
		},
		Spec: v1.ContextSpec{
			Type:     v1.ContextTypeFile,
			FilePath: req.FilePath,
			Content:  req.Content,
		},
	}

	body, _ := json.Marshal(ctx)
	resp, err := a.client.Post(a.apiServerURL+"/api/v1/contexts", "application/json", bytes.NewReader(body))
	if err != nil {
		a.writeJSON(w, http.StatusInternalServerError, LegacyContextShareResponse{
			Success: false,
			Message: "failed to share context: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		a.writeJSON(w, resp.StatusCode, LegacyContextShareResponse{
			Success: false,
			Message: string(respBody),
		})
		return
	}

	a.writeJSON(w, http.StatusOK, LegacyContextShareResponse{
		Success: true,
		Message: "context shared",
	})
}

func (a *LegacyAdapter) searchContext(w http.ResponseWriter, r *http.Request) {
	var req LegacyContextSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.writeJSON(w, http.StatusBadRequest, LegacyContextSearchResponse{
			Success: false,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	// For now, just list all contexts
	// Full search would require vector DB integration
	resp, err := a.client.Get(a.apiServerURL + "/api/v1/contexts")
	if err != nil {
		a.writeJSON(w, http.StatusInternalServerError, LegacyContextSearchResponse{
			Success: false,
			Message: "failed to search context: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	var list v1.ContextList
	json.NewDecoder(resp.Body).Decode(&list)

	limit := req.Limit
	if limit == 0 || limit > len(list.Items) {
		limit = len(list.Items)
	}

	results := make([]LegacyContextSearchResult, 0, limit)
	for i := 0; i < limit; i++ {
		ctx := list.Items[i]
		results = append(results, LegacyContextSearchResult{
			FilePath: ctx.Spec.FilePath,
			Content:  ctx.Spec.Content,
			Score:    1.0, // Placeholder
		})
	}

	a.writeJSON(w, http.StatusOK, LegacyContextSearchResponse{
		Success: true,
		Results: results,
	})
}

func (a *LegacyAdapter) listAgents(w http.ResponseWriter, r *http.Request) {
	resp, err := a.client.Get(a.apiServerURL + "/api/v1/agents")
	if err != nil {
		a.writeJSON(w, http.StatusInternalServerError, LegacyAgentsResponse{
			Success: false,
			Message: "failed to list agents: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	var list v1.AgentList
	json.NewDecoder(resp.Body).Decode(&list)

	agents := make([]LegacyAgentInfo, 0, len(list.Items))
	for _, agent := range list.Items {
		caps := make([]string, len(agent.Spec.Capabilities))
		for i, c := range agent.Spec.Capabilities {
			caps[i] = string(c)
		}
		agents = append(agents, LegacyAgentInfo{
			ID:           agent.Name,
			Name:         agent.Spec.DisplayName,
			Provider:     string(agent.Spec.Provider),
			Capabilities: caps,
			Status:       string(agent.Status.Phase),
		})
	}

	a.writeJSON(w, http.StatusOK, LegacyAgentsResponse{
		Success: true,
		Agents:  agents,
	})
}

func (a *LegacyAdapter) getStatus(w http.ResponseWriter, r *http.Request) {
	// Check health
	resp, err := a.client.Get(a.apiServerURL + "/readyz")
	if err != nil {
		a.writeJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"success": false,
			"message": "failed to get status: " + err.Error(),
		})
		return
	}
	defer resp.Body.Close()

	a.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success": resp.StatusCode == http.StatusOK,
		"status":  "running",
	})
}

func (a *LegacyAdapter) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// Helper functions

func generateLockName(filePath string, startLine, endLine int) string {
	if startLine > 0 {
		return fmt.Sprintf("lock-%x-%d-%d", hashString(filePath), startLine, endLine)
	}
	return fmt.Sprintf("lock-%x", hashString(filePath))
}

func generateContextName(filePath string) string {
	return fmt.Sprintf("ctx-%x-%d", hashString(filePath), time.Now().UnixNano())
}

func hashString(s string) uint32 {
	h := uint32(0)
	for _, c := range s {
		h = h*31 + uint32(c)
	}
	return h
}
