package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/google/uuid"
)

// DefaultEventSocketPath returns the default Unix socket path for events.
func DefaultEventSocketPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".agent-collab", "events.sock")
}

// EventServer streams events to connected clients via Unix socket.
type EventServer struct {
	mu sync.RWMutex

	socketPath string
	listener   net.Listener
	bus        *EventBus

	clients   map[string]net.Conn
	clientsMu sync.RWMutex

	running atomic.Bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// NewEventServer creates a new event server.
func NewEventServer(bus *EventBus) *EventServer {
	return &EventServer{
		socketPath: DefaultEventSocketPath(),
		bus:        bus,
		clients:    make(map[string]net.Conn),
	}
}

// Start starts the event server.
func (s *EventServer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.ctx, s.cancel = context.WithCancel(ctx)

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
		return fmt.Errorf("failed to create event socket: %w", err)
	}
	s.listener = listener

	// Set socket permissions
	os.Chmod(s.socketPath, 0600)

	s.running.Store(true)

	// Accept connections
	go s.acceptLoop()

	return nil
}

// Stop stops the event server.
func (s *EventServer) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.running.Store(false)

	if s.cancel != nil {
		s.cancel()
	}

	// Close all client connections
	s.clientsMu.Lock()
	for id, conn := range s.clients {
		conn.Close()
		delete(s.clients, id)
	}
	s.clientsMu.Unlock()

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Clean up socket file
	os.Remove(s.socketPath)

	return nil
}

// ClientCount returns the number of connected clients.
func (s *EventServer) ClientCount() int {
	s.clientsMu.RLock()
	defer s.clientsMu.RUnlock()
	return len(s.clients)
}

func (s *EventServer) acceptLoop() {
	for s.running.Load() {
		conn, err := s.listener.Accept()
		if err != nil {
			if s.running.Load() {
				fmt.Fprintf(os.Stderr, "Event server accept error: %v\n", err)
			}
			continue
		}

		clientID := uuid.New().String()
		s.clientsMu.Lock()
		s.clients[clientID] = conn
		s.clientsMu.Unlock()

		go s.handleClient(clientID, conn)
	}
}

func (s *EventServer) handleClient(clientID string, conn net.Conn) {
	defer func() {
		conn.Close()
		s.clientsMu.Lock()
		delete(s.clients, clientID)
		s.clientsMu.Unlock()
	}()

	// Subscribe to events
	eventCh := s.bus.Subscribe(clientID)
	defer s.bus.Unsubscribe(clientID)

	// Send ready event
	readyEvent := NewEvent(EventDaemonReady, nil)
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(readyEvent); err != nil {
		return
	}

	// Stream events to client
	for {
		select {
		case <-s.ctx.Done():
			return
		case event, ok := <-eventCh:
			if !ok {
				return
			}
			if err := encoder.Encode(event); err != nil {
				return
			}
		}
	}
}

// Publish publishes an event to all connected clients.
func (s *EventServer) Publish(event Event) {
	s.bus.Publish(event)
}
