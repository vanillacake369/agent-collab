// Package apiserver provides a RESTful API server for resources.
package apiserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	v1 "agent-collab/src/api/v1"
	"agent-collab/src/store"
)

// Server provides RESTful API endpoints for resources.
type Server struct {
	lockStore    store.ResourceStore[LockObject]
	contextStore store.ResourceStore[ContextObject]
	agentStore   store.ResourceStore[AgentObject]

	mux    *http.ServeMux
	server *http.Server

	mu sync.RWMutex
}

// LockObject is the interface for Lock store objects.
type LockObject interface {
	store.Object
	GetLock() *v1.Lock
}

// ContextObject is the interface for Context store objects.
type ContextObject interface {
	store.Object
	GetContext() *v1.Context
}

// AgentObject is the interface for Agent store objects.
type AgentObject interface {
	store.Object
	GetAgent() *v1.Agent
}

// NewServer creates a new API server.
func NewServer() *Server {
	s := &Server{
		mux: http.NewServeMux(),
	}
	s.registerRoutes()
	return s
}

// SetLockStore sets the lock store.
func (s *Server) SetLockStore(store store.ResourceStore[LockObject]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lockStore = store
}

// SetContextStore sets the context store.
func (s *Server) SetContextStore(store store.ResourceStore[ContextObject]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.contextStore = store
}

// SetAgentStore sets the agent store.
func (s *Server) SetAgentStore(store store.ResourceStore[AgentObject]) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.agentStore = store
}

func (s *Server) registerRoutes() {
	// Lock API
	s.mux.HandleFunc("GET /api/v1/locks", s.listLocks)
	s.mux.HandleFunc("POST /api/v1/locks", s.createLock)
	s.mux.HandleFunc("GET /api/v1/locks/{name}", s.getLock)
	s.mux.HandleFunc("PUT /api/v1/locks/{name}", s.updateLock)
	s.mux.HandleFunc("DELETE /api/v1/locks/{name}", s.deleteLock)

	// Context API
	s.mux.HandleFunc("GET /api/v1/contexts", s.listContexts)
	s.mux.HandleFunc("POST /api/v1/contexts", s.createContext)
	s.mux.HandleFunc("GET /api/v1/contexts/{name}", s.getContext)
	s.mux.HandleFunc("PUT /api/v1/contexts/{name}", s.updateContext)
	s.mux.HandleFunc("DELETE /api/v1/contexts/{name}", s.deleteContext)

	// Agent API
	s.mux.HandleFunc("GET /api/v1/agents", s.listAgents)
	s.mux.HandleFunc("POST /api/v1/agents", s.createAgent)
	s.mux.HandleFunc("GET /api/v1/agents/{name}", s.getAgent)
	s.mux.HandleFunc("PUT /api/v1/agents/{name}", s.updateAgent)
	s.mux.HandleFunc("DELETE /api/v1/agents/{name}", s.deleteAgent)

	// Watch endpoints (SSE)
	s.mux.HandleFunc("GET /api/v1/watch/locks", s.watchLocks)
	s.mux.HandleFunc("GET /api/v1/watch/contexts", s.watchContexts)
	s.mux.HandleFunc("GET /api/v1/watch/agents", s.watchAgents)

	// Health check
	s.mux.HandleFunc("GET /healthz", s.healthz)
	s.mux.HandleFunc("GET /readyz", s.readyz)
}

// Handler returns the HTTP handler.
func (s *Server) Handler() http.Handler {
	return s.mux
}

// Start starts the server on the given address.
func (s *Server) Start(addr string) error {
	s.server = &http.Server{
		Addr:    addr,
		Handler: s.mux,
	}
	return s.server.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.server != nil {
		return s.server.Shutdown(ctx)
	}
	return nil
}

// Lock handlers

func (s *Server) listLocks(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	lockStore := s.lockStore
	s.mu.RUnlock()

	if lockStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "lock store not configured")
		return
	}

	locks, err := lockStore.List(r.Context(), store.ListOptions{})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Convert to Lock list
	items := make([]v1.Lock, 0, len(locks))
	for _, l := range locks {
		items = append(items, *l.GetLock())
	}

	list := v1.LockList{
		TypeMeta: v1.TypeMeta{Kind: "LockList", APIVersion: v1.GroupVersion},
		Items:    items,
	}

	s.writeJSON(w, http.StatusOK, list)
}

func (s *Server) createLock(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	lockStore := s.lockStore
	s.mu.RUnlock()

	if lockStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "lock store not configured")
		return
	}

	var lock v1.Lock
	if err := json.NewDecoder(r.Body).Decode(&lock); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	// Set defaults
	lock.Kind = v1.LockKind
	lock.APIVersion = v1.GroupVersion
	if lock.CreationTimestamp.IsZero() {
		lock.CreationTimestamp = time.Now()
	}
	if lock.Status.Phase == "" {
		lock.Status.Phase = v1.LockPhasePending
	}

	// Create wrapper that implements LockObject
	wrapper := &lockObjectWrapper{Lock: &lock}

	if err := lockStore.Create(r.Context(), wrapper); err != nil {
		if err == store.ErrAlreadyExists {
			s.writeError(w, http.StatusConflict, "lock already exists")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, lock)
}

func (s *Server) getLock(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.mu.RLock()
	lockStore := s.lockStore
	s.mu.RUnlock()

	if lockStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "lock store not configured")
		return
	}

	lock, err := lockStore.Get(r.Context(), name)
	if err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "lock not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, lock.GetLock())
}

func (s *Server) updateLock(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.mu.RLock()
	lockStore := s.lockStore
	s.mu.RUnlock()

	if lockStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "lock store not configured")
		return
	}

	var lock v1.Lock
	if err := json.NewDecoder(r.Body).Decode(&lock); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	lock.Name = name
	wrapper := &lockObjectWrapper{Lock: &lock}

	if err := lockStore.Update(r.Context(), wrapper); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "lock not found")
			return
		}
		if err == store.ErrConflict {
			s.writeError(w, http.StatusConflict, "resource version conflict")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, lock)
}

func (s *Server) deleteLock(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.mu.RLock()
	lockStore := s.lockStore
	s.mu.RUnlock()

	if lockStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "lock store not configured")
		return
	}

	if err := lockStore.Delete(r.Context(), name); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "lock not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, v1.Status{
		TypeMeta: v1.TypeMeta{Kind: "Status", APIVersion: v1.GroupVersion},
		Status:   "Success",
		Message:  "lock deleted",
	})
}

// Context handlers

func (s *Server) listContexts(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	contextStore := s.contextStore
	s.mu.RUnlock()

	if contextStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "context store not configured")
		return
	}

	contexts, err := contextStore.List(r.Context(), store.ListOptions{})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]v1.Context, 0, len(contexts))
	for _, c := range contexts {
		items = append(items, *c.GetContext())
	}

	list := v1.ContextList{
		TypeMeta: v1.TypeMeta{Kind: "ContextList", APIVersion: v1.GroupVersion},
		Items:    items,
	}

	s.writeJSON(w, http.StatusOK, list)
}

func (s *Server) createContext(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	contextStore := s.contextStore
	s.mu.RUnlock()

	if contextStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "context store not configured")
		return
	}

	var ctx v1.Context
	if err := json.NewDecoder(r.Body).Decode(&ctx); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ctx.Kind = v1.ContextKind
	ctx.APIVersion = v1.GroupVersion
	if ctx.CreationTimestamp.IsZero() {
		ctx.CreationTimestamp = time.Now()
	}
	if ctx.Status.Phase == "" {
		ctx.Status.Phase = v1.ContextPhasePending
	}

	wrapper := &contextObjectWrapper{Context: &ctx}

	if err := contextStore.Create(r.Context(), wrapper); err != nil {
		if err == store.ErrAlreadyExists {
			s.writeError(w, http.StatusConflict, "context already exists")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, ctx)
}

func (s *Server) getContext(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.mu.RLock()
	contextStore := s.contextStore
	s.mu.RUnlock()

	if contextStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "context store not configured")
		return
	}

	ctx, err := contextStore.Get(r.Context(), name)
	if err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "context not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, ctx.GetContext())
}

func (s *Server) updateContext(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.mu.RLock()
	contextStore := s.contextStore
	s.mu.RUnlock()

	if contextStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "context store not configured")
		return
	}

	var ctx v1.Context
	if err := json.NewDecoder(r.Body).Decode(&ctx); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	ctx.Name = name
	wrapper := &contextObjectWrapper{Context: &ctx}

	if err := contextStore.Update(r.Context(), wrapper); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "context not found")
			return
		}
		if err == store.ErrConflict {
			s.writeError(w, http.StatusConflict, "resource version conflict")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, ctx)
}

func (s *Server) deleteContext(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.mu.RLock()
	contextStore := s.contextStore
	s.mu.RUnlock()

	if contextStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "context store not configured")
		return
	}

	if err := contextStore.Delete(r.Context(), name); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "context not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, v1.Status{
		TypeMeta: v1.TypeMeta{Kind: "Status", APIVersion: v1.GroupVersion},
		Status:   "Success",
		Message:  "context deleted",
	})
}

// Agent handlers

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	agentStore := s.agentStore
	s.mu.RUnlock()

	if agentStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "agent store not configured")
		return
	}

	agents, err := agentStore.List(r.Context(), store.ListOptions{})
	if err != nil {
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	items := make([]v1.Agent, 0, len(agents))
	for _, a := range agents {
		items = append(items, *a.GetAgent())
	}

	list := v1.AgentList{
		TypeMeta: v1.TypeMeta{Kind: "AgentList", APIVersion: v1.GroupVersion},
		Items:    items,
	}

	s.writeJSON(w, http.StatusOK, list)
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	agentStore := s.agentStore
	s.mu.RUnlock()

	if agentStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "agent store not configured")
		return
	}

	var agent v1.Agent
	if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	agent.Kind = v1.AgentKind
	agent.APIVersion = v1.GroupVersion
	if agent.CreationTimestamp.IsZero() {
		agent.CreationTimestamp = time.Now()
	}
	if agent.Status.Phase == "" {
		agent.Status.Phase = v1.AgentPhasePending
	}

	wrapper := &agentObjectWrapper{Agent: &agent}

	if err := agentStore.Create(r.Context(), wrapper); err != nil {
		if err == store.ErrAlreadyExists {
			s.writeError(w, http.StatusConflict, "agent already exists")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusCreated, agent)
}

func (s *Server) getAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.mu.RLock()
	agentStore := s.agentStore
	s.mu.RUnlock()

	if agentStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "agent store not configured")
		return
	}

	agent, err := agentStore.Get(r.Context(), name)
	if err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, agent.GetAgent())
}

func (s *Server) updateAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.mu.RLock()
	agentStore := s.agentStore
	s.mu.RUnlock()

	if agentStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "agent store not configured")
		return
	}

	var agent v1.Agent
	if err := json.NewDecoder(r.Body).Decode(&agent); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}

	agent.Name = name
	wrapper := &agentObjectWrapper{Agent: &agent}

	if err := agentStore.Update(r.Context(), wrapper); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		if err == store.ErrConflict {
			s.writeError(w, http.StatusConflict, "resource version conflict")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, agent)
}

func (s *Server) deleteAgent(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if name == "" {
		s.writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	s.mu.RLock()
	agentStore := s.agentStore
	s.mu.RUnlock()

	if agentStore == nil {
		s.writeError(w, http.StatusServiceUnavailable, "agent store not configured")
		return
	}

	if err := agentStore.Delete(r.Context(), name); err != nil {
		if err == store.ErrNotFound {
			s.writeError(w, http.StatusNotFound, "agent not found")
			return
		}
		s.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	s.writeJSON(w, http.StatusOK, v1.Status{
		TypeMeta: v1.TypeMeta{Kind: "Status", APIVersion: v1.GroupVersion},
		Status:   "Success",
		Message:  "agent deleted",
	})
}

// Watch handlers (SSE)

func (s *Server) watchLocks(w http.ResponseWriter, r *http.Request) {
	s.watchResource(w, r, "lock", func(ctx context.Context, opts store.WatchOptions) (interface{}, error) {
		s.mu.RLock()
		lockStore := s.lockStore
		s.mu.RUnlock()
		if lockStore == nil {
			return nil, fmt.Errorf("lock store not configured")
		}
		return lockStore.Watch(ctx, opts)
	})
}

func (s *Server) watchContexts(w http.ResponseWriter, r *http.Request) {
	s.watchResource(w, r, "context", func(ctx context.Context, opts store.WatchOptions) (interface{}, error) {
		s.mu.RLock()
		contextStore := s.contextStore
		s.mu.RUnlock()
		if contextStore == nil {
			return nil, fmt.Errorf("context store not configured")
		}
		return contextStore.Watch(ctx, opts)
	})
}

func (s *Server) watchAgents(w http.ResponseWriter, r *http.Request) {
	s.watchResource(w, r, "agent", func(ctx context.Context, opts store.WatchOptions) (interface{}, error) {
		s.mu.RLock()
		agentStore := s.agentStore
		s.mu.RUnlock()
		if agentStore == nil {
			return nil, fmt.Errorf("agent store not configured")
		}
		return agentStore.Watch(ctx, opts)
	})
}

func (s *Server) watchResource(w http.ResponseWriter, r *http.Request, resourceType string, watchFn func(context.Context, store.WatchOptions) (interface{}, error)) {
	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		s.writeError(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	// Parse options
	opts := store.WatchOptions{
		SendInitialEvents: r.URL.Query().Get("sendInitialEvents") == "true",
		ResourceVersion:   r.URL.Query().Get("resourceVersion"),
	}

	// Parse label selector
	if labelSelector := r.URL.Query().Get("labelSelector"); labelSelector != "" {
		opts.LabelSelector = parseLabelSelector(labelSelector)
	}

	watcher, err := watchFn(r.Context(), opts)
	if err != nil {
		fmt.Fprintf(w, "event: error\ndata: %s\n\n", err.Error())
		flusher.Flush()
		return
	}

	// Type-specific handling would go here
	// For now, just send a connected message
	fmt.Fprintf(w, "event: connected\ndata: {\"resourceType\":\"%s\"}\n\n", resourceType)
	flusher.Flush()

	// Keep connection alive
	<-r.Context().Done()

	// Close watcher based on type
	switch w := watcher.(type) {
	case store.Watcher[LockObject]:
		w.Stop()
	case store.Watcher[ContextObject]:
		w.Stop()
	case store.Watcher[AgentObject]:
		w.Stop()
	}
}

// Health handlers

func (s *Server) healthz(w http.ResponseWriter, r *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	// Check if stores are configured
	s.mu.RLock()
	ready := s.lockStore != nil && s.contextStore != nil && s.agentStore != nil
	s.mu.RUnlock()

	if ready {
		s.writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	} else {
		s.writeError(w, http.StatusServiceUnavailable, "stores not configured")
	}
}

// Helper methods

func (s *Server) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	s.writeJSON(w, status, v1.Status{
		TypeMeta: v1.TypeMeta{Kind: "Status", APIVersion: v1.GroupVersion},
		Status:   "Failure",
		Message:  message,
		Code:     int32(status),
	})
}

func parseLabelSelector(selector string) map[string]string {
	result := make(map[string]string)
	pairs := strings.Split(selector, ",")
	for _, pair := range pairs {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 {
			result[kv[0]] = kv[1]
		}
	}
	return result
}

// Object wrappers for type safety

type lockObjectWrapper struct {
	*v1.Lock
}

func (l *lockObjectWrapper) GetLock() *v1.Lock {
	return l.Lock
}

func (l *lockObjectWrapper) GetObjectMeta() *v1.ObjectMeta {
	return &l.ObjectMeta
}

func (l *lockObjectWrapper) GetTypeMeta() *v1.TypeMeta {
	return &l.TypeMeta
}

func (l *lockObjectWrapper) DeepCopy() store.Object {
	if l == nil || l.Lock == nil {
		return nil
	}
	copy := *l.Lock
	return &lockObjectWrapper{Lock: &copy}
}

type contextObjectWrapper struct {
	*v1.Context
}

func (c *contextObjectWrapper) GetContext() *v1.Context {
	return c.Context
}

func (c *contextObjectWrapper) GetObjectMeta() *v1.ObjectMeta {
	return &c.ObjectMeta
}

func (c *contextObjectWrapper) GetTypeMeta() *v1.TypeMeta {
	return &c.TypeMeta
}

func (c *contextObjectWrapper) DeepCopy() store.Object {
	if c == nil || c.Context == nil {
		return nil
	}
	copy := *c.Context
	return &contextObjectWrapper{Context: &copy}
}

type agentObjectWrapper struct {
	*v1.Agent
}

func (a *agentObjectWrapper) GetAgent() *v1.Agent {
	return a.Agent
}

func (a *agentObjectWrapper) GetObjectMeta() *v1.ObjectMeta {
	return &a.ObjectMeta
}

func (a *agentObjectWrapper) GetTypeMeta() *v1.TypeMeta {
	return &a.TypeMeta
}

func (a *agentObjectWrapper) DeepCopy() store.Object {
	if a == nil || a.Agent == nil {
		return nil
	}
	copy := *a.Agent
	return &agentObjectWrapper{Agent: &copy}
}
