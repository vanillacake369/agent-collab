// Package bootstrap provides application initialization for the new
// Resource/Controller architecture. It wires together all the components
// and starts the controller manager.
package bootstrap

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	v1 "agent-collab/src/api/v1"
	"agent-collab/src/apiserver"
	"agent-collab/src/controller"
	"agent-collab/src/plugin"
	"agent-collab/src/store"
	"agent-collab/src/store/memory"

	embeddingPlugin "agent-collab/src/plugin/embedding"
	libp2pPlugin "agent-collab/src/plugin/network/libp2p"
)

// Config holds the bootstrap configuration.
type Config struct {
	// ProjectID is the project identifier for namespacing.
	ProjectID string

	// AgentID is the local agent identifier.
	AgentID string

	// APIServerAddr is the address for the API server (e.g., ":8080").
	APIServerAddr string

	// NetworkConfig is the libp2p configuration.
	NetworkConfig *libp2pPlugin.Config

	// EmbeddingConfig is the embedding configuration.
	EmbeddingConfig *embeddingPlugin.Config

	// EnableLegacyAPI enables the legacy API adapter.
	EnableLegacyAPI bool
}

// Application represents the bootstrapped application.
type Application struct {
	config *Config

	// Stores
	lockStore    *memory.Store[*lockObject]
	contextStore *memory.Store[*contextObject]
	agentStore   *memory.Store[*agentObject]

	// Plugins
	networkPlugin   plugin.NetworkPlugin
	embeddingPlugin plugin.EmbeddingPlugin

	// Manager
	controllerMgr *controller.Manager
	apiServer     *apiserver.Server

	// Lifecycle
	mu      sync.Mutex
	started bool
	stopCh  chan struct{}
}

// Store object wrappers

type lockObject struct {
	*v1.Lock
}

func (l *lockObject) DeepCopy() store.Object {
	if l == nil || l.Lock == nil {
		return nil
	}
	copy := *l.Lock
	return &lockObject{Lock: &copy}
}

type contextObject struct {
	*v1.Context
}

func (c *contextObject) DeepCopy() store.Object {
	if c == nil || c.Context == nil {
		return nil
	}
	copy := *c.Context
	return &contextObject{Context: &copy}
}

type agentObject struct {
	*v1.Agent
}

func (a *agentObject) DeepCopy() store.Object {
	if a == nil || a.Agent == nil {
		return nil
	}
	copy := *a.Agent
	return &agentObject{Agent: &copy}
}

// NewApplication creates a new application with the given configuration.
func NewApplication(cfg *Config) *Application {
	return &Application{
		config: cfg,
		stopCh: make(chan struct{}),
	}
}

// Initialize initializes all components.
func (app *Application) Initialize(ctx context.Context) error {
	app.mu.Lock()
	defer app.mu.Unlock()

	// Initialize stores
	app.lockStore = memory.New[*lockObject]()
	app.contextStore = memory.New[*contextObject]()
	app.agentStore = memory.New[*agentObject]()

	// Initialize plugins
	if app.config.NetworkConfig != nil {
		app.networkPlugin = libp2pPlugin.NewPlugin(app.config.NetworkConfig)
	}
	if app.config.EmbeddingConfig != nil {
		app.embeddingPlugin = embeddingPlugin.NewPlugin(app.config.EmbeddingConfig)
	}

	// Initialize controller manager
	app.controllerMgr = controller.NewManager()
	if app.networkPlugin != nil {
		app.controllerMgr.SetNetwork(app.networkPlugin)
	}

	// Initialize API server
	app.apiServer = apiserver.NewServer()

	return nil
}

// Start starts all components.
func (app *Application) Start(ctx context.Context) error {
	app.mu.Lock()
	if app.started {
		app.mu.Unlock()
		return fmt.Errorf("application already started")
	}
	app.started = true
	app.mu.Unlock()

	var wg sync.WaitGroup
	errCh := make(chan error, 5)

	// Start plugins
	if app.networkPlugin != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := app.networkPlugin.Start(ctx); err != nil {
				errCh <- fmt.Errorf("network plugin: %w", err)
			}
		}()
	}

	if app.embeddingPlugin != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := app.embeddingPlugin.Start(ctx); err != nil {
				errCh <- fmt.Errorf("embedding plugin: %w", err)
			}
		}()
	}

	// Wait for plugins to initialize
	select {
	case err := <-errCh:
		return err
	case <-time.After(5 * time.Second):
	}

	// Start API server
	if app.config.APIServerAddr != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := app.apiServer.Start(app.config.APIServerAddr); err != nil && err != http.ErrServerClosed {
				errCh <- fmt.Errorf("api server: %w", err)
			}
		}()
	}

	// Start controller manager
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := app.controllerMgr.Start(ctx); err != nil {
			errCh <- fmt.Errorf("controller manager: %w", err)
		}
	}()

	// Wait for context cancellation or error
	select {
	case <-ctx.Done():
		// Graceful shutdown
		app.Stop()
		wg.Wait()
		return ctx.Err()
	case err := <-errCh:
		app.Stop()
		wg.Wait()
		return err
	}
}

// Stop stops all components.
func (app *Application) Stop() error {
	app.mu.Lock()
	defer app.mu.Unlock()

	close(app.stopCh)

	var lastErr error

	// Stop API server
	if app.apiServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := app.apiServer.Shutdown(ctx); err != nil {
			lastErr = err
		}
		cancel()
	}

	// Stop plugins
	if app.networkPlugin != nil {
		if err := app.networkPlugin.Stop(); err != nil {
			lastErr = err
		}
	}
	if app.embeddingPlugin != nil {
		if err := app.embeddingPlugin.Stop(); err != nil {
			lastErr = err
		}
	}

	// Close stores
	if app.lockStore != nil {
		app.lockStore.Close()
	}
	if app.contextStore != nil {
		app.contextStore.Close()
	}
	if app.agentStore != nil {
		app.agentStore.Close()
	}

	return lastErr
}

// Getters

// LockStore returns the lock store.
func (app *Application) LockStore() *memory.Store[*lockObject] {
	return app.lockStore
}

// ContextStore returns the context store.
func (app *Application) ContextStore() *memory.Store[*contextObject] {
	return app.contextStore
}

// AgentStore returns the agent store.
func (app *Application) AgentStore() *memory.Store[*agentObject] {
	return app.agentStore
}

// NetworkPlugin returns the network plugin.
func (app *Application) NetworkPlugin() plugin.NetworkPlugin {
	return app.networkPlugin
}

// EmbeddingPlugin returns the embedding plugin.
func (app *Application) EmbeddingPlugin() plugin.EmbeddingPlugin {
	return app.embeddingPlugin
}

// ControllerManager returns the controller manager.
func (app *Application) ControllerManager() *controller.Manager {
	return app.controllerMgr
}

// APIServer returns the API server.
func (app *Application) APIServer() *apiserver.Server {
	return app.apiServer
}
