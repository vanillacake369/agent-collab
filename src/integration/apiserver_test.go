package integration

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	v1 "agent-collab/src/api/v1"
	"agent-collab/src/apiserver"
	"agent-collab/src/store"
	"agent-collab/src/store/memory"
)

// Wrapper types that implement apiserver interfaces
type lockObjectAPI struct {
	*v1.Lock
}

func (l *lockObjectAPI) GetLock() *v1.Lock             { return l.Lock }
func (l *lockObjectAPI) GetObjectMeta() *v1.ObjectMeta { return &l.ObjectMeta }
func (l *lockObjectAPI) GetTypeMeta() *v1.TypeMeta     { return &l.TypeMeta }
func (l *lockObjectAPI) DeepCopy() store.Object {
	if l == nil || l.Lock == nil {
		return nil
	}
	copy := *l.Lock
	return &lockObjectAPI{Lock: &copy}
}

type contextObjectAPI struct {
	*v1.Context
}

func (c *contextObjectAPI) GetContext() *v1.Context       { return c.Context }
func (c *contextObjectAPI) GetObjectMeta() *v1.ObjectMeta { return &c.ObjectMeta }
func (c *contextObjectAPI) GetTypeMeta() *v1.TypeMeta     { return &c.TypeMeta }
func (c *contextObjectAPI) DeepCopy() store.Object {
	if c == nil || c.Context == nil {
		return nil
	}
	copy := *c.Context
	return &contextObjectAPI{Context: &copy}
}

type agentObjectAPI struct {
	*v1.Agent
}

func (a *agentObjectAPI) GetAgent() *v1.Agent           { return a.Agent }
func (a *agentObjectAPI) GetObjectMeta() *v1.ObjectMeta { return &a.ObjectMeta }
func (a *agentObjectAPI) GetTypeMeta() *v1.TypeMeta     { return &a.TypeMeta }
func (a *agentObjectAPI) DeepCopy() store.Object {
	if a == nil || a.Agent == nil {
		return nil
	}
	copy := *a.Agent
	return &agentObjectAPI{Agent: &copy}
}

func TestAPIServerLockEndpoints(t *testing.T) {
	// Create server and stores
	server := apiserver.NewServer()
	lockStore := memory.New[apiserver.LockObject]()
	defer lockStore.Close()

	server.SetLockStore(lockStore)

	// Create test server
	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	client := ts.Client()

	t.Run("CreateLock", func(t *testing.T) {
		lock := v1.Lock{
			ObjectMeta: v1.ObjectMeta{
				Name: "api-lock-1",
			},
			Spec: v1.LockSpec{
				Target: v1.LockTarget{
					Type:     v1.LockTargetTypeFile,
					FilePath: "/path/to/file.go",
				},
				HolderID:  "agent-1",
				Intention: "API test",
				TTL:       v1.Duration{Duration: 5 * time.Minute},
				Exclusive: true,
			},
		}

		body, _ := json.Marshal(lock)
		resp, err := client.Post(ts.URL+"/api/v1/locks", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusCreated)
		}

		var created v1.Lock
		if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		if created.Name != "api-lock-1" {
			t.Errorf("Name = %s, want api-lock-1", created.Name)
		}
		if created.ResourceVersion == "" {
			t.Error("ResourceVersion should be set")
		}
	})

	t.Run("GetLock", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/v1/locks/api-lock-1")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var lock v1.Lock
		if err := json.NewDecoder(resp.Body).Decode(&lock); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		if lock.Spec.HolderID != "agent-1" {
			t.Errorf("HolderID = %s, want agent-1", lock.Spec.HolderID)
		}
	})

	t.Run("ListLocks", func(t *testing.T) {
		// Create another lock
		lock2 := v1.Lock{
			ObjectMeta: v1.ObjectMeta{
				Name: "api-lock-2",
			},
			Spec: v1.LockSpec{
				HolderID: "agent-2",
				TTL:      v1.Duration{Duration: time.Minute},
			},
		}
		body, _ := json.Marshal(lock2)
		client.Post(ts.URL+"/api/v1/locks", "application/json", bytes.NewReader(body))

		resp, err := client.Get(ts.URL + "/api/v1/locks")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var list v1.LockList
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		if len(list.Items) != 2 {
			t.Errorf("Items count = %d, want 2", len(list.Items))
		}
	})

	t.Run("DeleteLock", func(t *testing.T) {
		req, _ := http.NewRequest("DELETE", ts.URL+"/api/v1/locks/api-lock-1", nil)
		resp, err := client.Do(req)
		if err != nil {
			t.Fatalf("DELETE failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		// Verify deletion
		resp, _ = client.Get(ts.URL + "/api/v1/locks/api-lock-1")
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("After delete status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
	})
}

func TestAPIServerContextEndpoints(t *testing.T) {
	server := apiserver.NewServer()
	contextStore := memory.New[apiserver.ContextObject]()
	defer contextStore.Close()

	server.SetContextStore(contextStore)

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	client := ts.Client()

	t.Run("CreateContext", func(t *testing.T) {
		ctx := v1.Context{
			ObjectMeta: v1.ObjectMeta{
				Name: "api-ctx-1",
			},
			Spec: v1.ContextSpec{
				Type:          v1.ContextTypeFile,
				SourceAgentID: "agent-1",
				FilePath:      "/path/to/file.go",
				Content:       "package main",
			},
		}

		body, _ := json.Marshal(ctx)
		resp, err := client.Post(ts.URL+"/api/v1/contexts", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusCreated)
		}
	})

	t.Run("GetContext", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/v1/contexts/api-ctx-1")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var ctx v1.Context
		if err := json.NewDecoder(resp.Body).Decode(&ctx); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		if ctx.Spec.Content != "package main" {
			t.Errorf("Content = %s, want 'package main'", ctx.Spec.Content)
		}
	})
}

func TestAPIServerAgentEndpoints(t *testing.T) {
	server := apiserver.NewServer()
	agentStore := memory.New[apiserver.AgentObject]()
	defer agentStore.Close()

	server.SetAgentStore(agentStore)

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	client := ts.Client()

	t.Run("CreateAgent", func(t *testing.T) {
		agent := v1.Agent{
			ObjectMeta: v1.ObjectMeta{
				Name: "api-agent-1",
			},
			Spec: v1.AgentSpec{
				Provider:    v1.AgentProviderAnthropic,
				Model:       "claude-3-opus",
				PeerID:      "12D3KooW...",
				DisplayName: "Test Agent",
				Capabilities: []v1.AgentCapability{
					v1.AgentCapabilityCodeEdit,
				},
			},
		}

		body, _ := json.Marshal(agent)
		resp, err := client.Post(ts.URL+"/api/v1/agents", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("POST failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusCreated {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusCreated)
		}
	})

	t.Run("ListAgents", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/api/v1/agents")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}

		var list v1.AgentList
		if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
			t.Fatalf("Decode failed: %v", err)
		}

		if len(list.Items) != 1 {
			t.Errorf("Items count = %d, want 1", len(list.Items))
		}

		if list.Items[0].Spec.Provider != v1.AgentProviderAnthropic {
			t.Errorf("Provider = %s, want Anthropic", list.Items[0].Spec.Provider)
		}
	})
}

func TestAPIServerHealthEndpoints(t *testing.T) {
	server := apiserver.NewServer()

	ts := httptest.NewServer(server.Handler())
	defer ts.Close()

	client := ts.Client()

	t.Run("Healthz", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/healthz")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	t.Run("ReadyzNotReady", func(t *testing.T) {
		resp, err := client.Get(ts.URL + "/readyz")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		// Should be not ready because stores aren't configured
		if resp.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusServiceUnavailable)
		}
	})

	t.Run("ReadyzReady", func(t *testing.T) {
		// Configure all stores
		lockStore := memory.New[apiserver.LockObject]()
		defer lockStore.Close()
		contextStore := memory.New[apiserver.ContextObject]()
		defer contextStore.Close()
		agentStore := memory.New[apiserver.AgentObject]()
		defer agentStore.Close()

		server.SetLockStore(lockStore)
		server.SetContextStore(contextStore)
		server.SetAgentStore(agentStore)

		resp, err := client.Get(ts.URL + "/readyz")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("Status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})
}
