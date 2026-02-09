package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"agent-collab/internal/application"
	"agent-collab/internal/domain/lock"
	"agent-collab/internal/infrastructure/storage/vector"
)

// Server is the daemon server that manages the agent-collab instance.
type Server struct {
	mu sync.RWMutex

	app        *application.App
	listener   net.Listener
	server     *http.Server
	socketPath string
	pidFile    string
	startedAt  time.Time

	// Event system
	eventBus    *EventBus
	eventServer *EventServer

	ctx    context.Context
	cancel context.CancelFunc
}

// DefaultSocketPath returns the default Unix socket path.
func DefaultSocketPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agent-collab", "daemon.sock")
}

// DefaultPIDFile returns the default PID file path.
func DefaultPIDFile() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agent-collab", "daemon.pid")
}

// NewServer creates a new daemon server.
func NewServer(app *application.App) *Server {
	eventBus := NewEventBus()
	return &Server{
		app:         app,
		socketPath:  DefaultSocketPath(),
		pidFile:     DefaultPIDFile(),
		eventBus:    eventBus,
		eventServer: NewEventServer(eventBus),
	}
}

// Start starts the daemon server.
func (s *Server) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ctx, s.cancel = context.WithCancel(ctx)
	s.startedAt = time.Now()

	// Ensure directory exists
	dir := filepath.Dir(s.socketPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Remove existing socket
	os.Remove(s.socketPath)

	// Create Unix socket listener
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	s.listener = listener

	// Set socket permissions
	os.Chmod(s.socketPath, 0600)

	// Write PID file
	if err := os.WriteFile(s.pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0600); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	// Create HTTP server with routes
	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
	}

	// Start the app
	if err := s.app.Start(); err != nil {
		return fmt.Errorf("failed to start app: %w", err)
	}

	// Start event server
	if err := s.eventServer.Start(s.ctx); err != nil {
		return fmt.Errorf("failed to start event server: %w", err)
	}

	// Serve in background
	go func() {
		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "Daemon server error: %v\n", err)
		}
	}()

	// Publish ready event
	s.PublishEvent(NewEvent(EventDaemonReady, nil))

	return nil
}

// Stop stops the daemon server.
func (s *Server) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Publish shutdown event
	s.eventBus.Publish(NewEvent(EventDaemonShutdown, nil))

	if s.cancel != nil {
		s.cancel()
	}

	// Stop event server
	if s.eventServer != nil {
		s.eventServer.Stop()
	}

	// Close event bus
	if s.eventBus != nil {
		s.eventBus.Close()
	}

	if s.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(ctx)
	}

	if s.listener != nil {
		s.listener.Close()
	}

	if s.app != nil {
		s.app.Stop()
	}

	// Clean up files
	os.Remove(s.socketPath)
	os.Remove(s.pidFile)

	return nil
}

// Wait blocks until the server is stopped.
func (s *Server) Wait() {
	<-s.ctx.Done()
}

// EventBus returns the event bus.
func (s *Server) EventBus() *EventBus {
	return s.eventBus
}

// PublishEvent publishes an event to all subscribers.
func (s *Server) PublishEvent(event Event) {
	s.eventBus.Publish(event)
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/status", s.handleStatus)
	mux.HandleFunc("/init", s.handleInit)
	mux.HandleFunc("/join", s.handleJoin)
	mux.HandleFunc("/lock/acquire", s.handleAcquireLock)
	mux.HandleFunc("/lock/release", s.handleReleaseLock)
	mux.HandleFunc("/lock/list", s.handleListLocks)
	mux.HandleFunc("/peers/list", s.handleListPeers)
	mux.HandleFunc("/embed", s.handleEmbed)
	mux.HandleFunc("/search", s.handleSearch)
	mux.HandleFunc("/agents/list", s.handleListAgents)
	mux.HandleFunc("/context/watch", s.handleWatchFile)
	mux.HandleFunc("/shutdown", s.handleShutdown)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := s.app.GetStatus()

	resp := StatusResponse{
		Running:     status.Running,
		PID:         os.Getpid(),
		StartedAt:   s.startedAt,
		ProjectName: status.ProjectName,
		NodeID:      status.NodeID,
		PeerCount:   status.PeerCount,
		LockCount:   status.LockCount,
	}

	if s.app.AgentRegistry() != nil {
		resp.AgentCount = s.app.AgentRegistry().Count()
	}

	if s.app.EmbeddingService() != nil {
		resp.EmbeddingProvider = string(s.app.EmbeddingService().Provider())
	}

	// Add event subscriber count
	resp.EventSubscribers = s.eventServer.ClientCount()

	json.NewEncoder(w).Encode(resp)
}

func (s *Server) handleInit(w http.ResponseWriter, r *http.Request) {
	var req InitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(InitResponse{Error: err.Error()})
		return
	}

	result, err := s.app.Initialize(s.ctx, req.ProjectName)
	if err != nil {
		json.NewEncoder(w).Encode(InitResponse{Error: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(InitResponse{
		Success:     true,
		ProjectName: result.ProjectName,
		NodeID:      result.NodeID,
		InviteToken: result.InviteToken,
	})
}

func (s *Server) handleJoin(w http.ResponseWriter, r *http.Request) {
	var req JoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(JoinResponse{Error: err.Error()})
		return
	}

	result, err := s.app.Join(s.ctx, req.Token)
	if err != nil {
		json.NewEncoder(w).Encode(JoinResponse{Error: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(JoinResponse{
		Success:        true,
		ProjectName:    result.ProjectName,
		NodeID:         result.NodeID,
		ConnectedPeers: result.ConnectedPeers,
	})
}

func (s *Server) handleAcquireLock(w http.ResponseWriter, r *http.Request) {
	var req LockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(LockResponse{Error: err.Error()})
		return
	}

	lockService := s.app.LockService()
	if lockService == nil {
		json.NewEncoder(w).Encode(LockResponse{Error: "lock service not initialized"})
		return
	}

	result, err := lockService.AcquireLock(s.ctx, &lock.AcquireLockRequest{
		TargetType: lock.TargetFile,
		FilePath:   req.FilePath,
		StartLine:  req.StartLine,
		EndLine:    req.EndLine,
		Intention:  req.Intention,
	})

	if err != nil {
		json.NewEncoder(w).Encode(LockResponse{Error: err.Error()})
		return
	}

	lockID := ""
	if result.Lock != nil {
		lockID = result.Lock.ID

		// Publish lock acquired event
		s.PublishEvent(NewEvent(EventLockAcquired, LockEventData{
			LockID:    lockID,
			FilePath:  req.FilePath,
			StartLine: req.StartLine,
			EndLine:   req.EndLine,
			AgentID:   result.Lock.HolderID,
			Intention: req.Intention,
		}))
	} else if !result.Success {
		// Publish lock conflict event
		s.PublishEvent(NewEvent(EventLockConflict, LockConflictData{
			FilePath:    req.FilePath,
			HolderID:    result.Reason,
			RequesterID: "unknown",
			Intention:   req.Intention,
		}))
	}

	json.NewEncoder(w).Encode(LockResponse{
		Success: result.Success,
		LockID:  lockID,
		Error:   result.Reason,
	})
}

func (s *Server) handleReleaseLock(w http.ResponseWriter, r *http.Request) {
	var req ReleaseLockRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: err.Error()})
		return
	}

	lockService := s.app.LockService()
	if lockService == nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: "lock service not initialized"})
		return
	}

	if err := lockService.ReleaseLock(s.ctx, req.LockID); err != nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: err.Error()})
		return
	}

	// Publish lock released event
	s.PublishEvent(NewEvent(EventLockReleased, LockEventData{
		LockID: req.LockID,
	}))

	json.NewEncoder(w).Encode(GenericResponse{Success: true, Message: "Lock released"})
}

func (s *Server) handleListLocks(w http.ResponseWriter, r *http.Request) {
	lockService := s.app.LockService()
	if lockService == nil {
		json.NewEncoder(w).Encode(ListLocksResponse{})
		return
	}

	locks := lockService.ListLocks()
	json.NewEncoder(w).Encode(ListLocksResponse{Locks: locks})
}

func (s *Server) handleListPeers(w http.ResponseWriter, r *http.Request) {
	node := s.app.Node()
	if node == nil {
		json.NewEncoder(w).Encode(ListPeersResponse{Peers: []PeerInfo{}})
		return
	}

	connectedPeers := node.ConnectedPeers()
	peers := make([]PeerInfo, 0, len(connectedPeers))

	for _, peerID := range connectedPeers {
		info := node.PeerInfo(peerID)
		addrs := make([]string, len(info.Addrs))
		for i, addr := range info.Addrs {
			addrs[i] = addr.String()
		}

		latency := node.Latency(peerID)

		peers = append(peers, PeerInfo{
			ID:        peerID.String(),
			Addresses: addrs,
			Latency:   latency.Milliseconds(),
			Connected: true,
		})
	}

	json.NewEncoder(w).Encode(ListPeersResponse{Peers: peers})
}

func (s *Server) handleEmbed(w http.ResponseWriter, r *http.Request) {
	var req EmbedRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: err.Error()})
		return
	}

	embedService := s.app.EmbeddingService()
	if embedService == nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: "embedding service not initialized"})
		return
	}

	embedding, err := embedService.Embed(s.ctx, req.Text)
	if err != nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: err.Error()})
		return
	}

	json.NewEncoder(w).Encode(EmbedResponse{
		Embedding: embedding,
		Dimension: embedService.Dimension(),
		Provider:  string(embedService.Provider()),
		Model:     embedService.Model(),
	})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	var req SearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: err.Error()})
		return
	}

	vectorStore := s.app.VectorStore()
	embedService := s.app.EmbeddingService()
	if vectorStore == nil || embedService == nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: "services not initialized"})
		return
	}

	// Generate embedding for query
	embedding, err := embedService.Embed(s.ctx, req.Query)
	if err != nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: err.Error()})
		return
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}

	results, err := vectorStore.Search(embedding, &vector.SearchOptions{
		Collection: "default",
		TopK:       limit,
	})
	if err != nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: err.Error()})
		return
	}

	// Convert results
	searchResults := make([]SearchResult, len(results))
	for i, r := range results {
		searchResults[i] = SearchResult{
			ID:      r.Document.ID,
			Content: r.Document.Content,
			Score:   r.Score,
		}
		if r.Document.Metadata != nil {
			searchResults[i].Metadata = r.Document.Metadata
		}
	}

	json.NewEncoder(w).Encode(SearchResponse{Results: searchResults})
}

func (s *Server) handleListAgents(w http.ResponseWriter, r *http.Request) {
	registry := s.app.AgentRegistry()
	if registry == nil {
		json.NewEncoder(w).Encode(ListAgentsResponse{})
		return
	}

	agents := registry.List()
	json.NewEncoder(w).Encode(ListAgentsResponse{Agents: agents})
}

func (s *Server) handleWatchFile(w http.ResponseWriter, r *http.Request) {
	var req WatchFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: err.Error()})
		return
	}

	syncManager := s.app.SyncManager()
	if syncManager == nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: "sync manager not initialized"})
		return
	}

	if err := syncManager.WatchFile(req.FilePath); err != nil {
		json.NewEncoder(w).Encode(GenericResponse{Error: err.Error()})
		return
	}

	// Publish context updated event
	s.PublishEvent(NewEvent(EventContextUpdated, ContextEventData{
		FilePath: req.FilePath,
	}))

	json.NewEncoder(w).Encode(GenericResponse{Success: true, Message: "Watching file"})
}

func (s *Server) handleShutdown(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(GenericResponse{Success: true, Message: "Shutting down"})
	go func() {
		time.Sleep(100 * time.Millisecond)
		s.Stop()
	}()
}
