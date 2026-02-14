package application

import (
	"context"
	"fmt"
	"os"

	"agent-collab/src/domain/agent"
	"agent-collab/src/domain/ctxsync"
	"agent-collab/src/domain/event"
	"agent-collab/src/domain/interest"
	"agent-collab/src/domain/lock"
	"agent-collab/src/domain/token"
	"agent-collab/src/infrastructure/crypto"
	"agent-collab/src/infrastructure/embedding"
	"agent-collab/src/infrastructure/network/libp2p"
	"agent-collab/src/infrastructure/network/wireguard"
	"agent-collab/src/infrastructure/storage/metrics"
	"agent-collab/src/infrastructure/storage/vector"
)

// GetStatus는 상태를 반환합니다.
func (a *App) GetStatus() *Status {
	a.mu.RLock()
	defer a.mu.RUnlock()

	status := &Status{
		Running:     a.running,
		ProjectName: a.config.ProjectName,
	}

	if a.node != nil {
		status.NodeID = a.node.ID().String()
		addrs := a.node.Addrs()
		status.Addresses = make([]string, len(addrs))
		for i, addr := range addrs {
			status.Addresses[i] = addr.String()
		}
		status.PeerCount = len(a.node.ConnectedPeers())
	}

	if a.lockService != nil {
		stats := a.lockService.GetStats()
		status.LockCount = stats.TotalLocks
		status.MyLockCount = stats.MyLocks
	}

	if a.syncManager != nil {
		stats := a.syncManager.GetStats()
		status.DeltaCount = stats.TotalDeltas
		status.WatchedFiles = stats.WatchedFiles
	}

	// Phase 3 metrics
	if a.tokenTracker != nil {
		metrics := a.tokenTracker.GetMetrics()
		status.TokensToday = metrics.TokensToday
		status.TokensPerHour = metrics.TokensPerHour
		status.CostToday = metrics.CostToday
	}

	if a.vectorStore != nil {
		if stats, err := a.vectorStore.GetCollectionStats("default"); err == nil {
			status.EmbeddingCount = stats.Count
		}
	}

	// WireGuard status
	if a.wgManager != nil {
		status.WireGuardEnabled = true
		status.WireGuardIP = a.wgManager.GetLocalIP()
		status.WireGuardEndpoint = a.wgManager.GetEndpoint()
		if wgStatus, err := a.wgManager.GetStatus(); err == nil {
			status.WireGuardPeerCount = len(wgStatus.Peers)
		}
	}

	return status
}

// LockService는 락 서비스를 반환합니다.
func (a *App) LockService() *lock.LockService {
	return a.lockService
}

// SyncManager는 동기화 관리자를 반환합니다.
func (a *App) SyncManager() *ctxsync.SyncManager {
	return a.syncManager
}

// Node는 libp2p 노드를 반환합니다.
func (a *App) Node() *libp2p.Node {
	return a.node
}

// KeyPair는 키 쌍을 반환합니다.
func (a *App) KeyPair() *crypto.KeyPair {
	return a.keyPair
}

// Config returns the application config.
func (a *App) Config() *Config {
	return a.config
}

// TokenTracker returns the token usage tracker.
func (a *App) TokenTracker() *token.Tracker {
	return a.tokenTracker
}

// VectorStore returns the vector store.
func (a *App) VectorStore() vector.Store {
	return a.vectorStore
}

// EmbeddingService returns the embedding service.
func (a *App) EmbeddingService() *embedding.Service {
	return a.embedService
}

// WireGuardManager returns the WireGuard VPN manager (nil if not enabled).
func (a *App) WireGuardManager() *wireguard.WireGuardManager {
	return a.wgManager
}

// AgentRegistry returns the agent registry.
func (a *App) AgentRegistry() *agent.Registry {
	return a.agentRegistry
}

// InterestManager returns the interest manager.
func (a *App) InterestManager() *interest.Manager {
	return a.interestMgr
}

// EventRouter returns the event router.
func (a *App) EventRouter() *event.Router {
	return a.eventRouter
}

// EventBridge returns the event bridge.
func (a *App) EventBridge() *libp2p.EventBridge {
	return a.eventBridge
}

// initPhase3Components initializes token tracking, vector storage, and embedding.
func (a *App) initPhase3Components(nodeID, nodeName string) error {
	// Initialize token tracker
	a.tokenTracker = token.NewTracker(nodeID, nodeName)

	// Initialize metrics store
	metricsStore, err := metrics.NewStore(a.config.DataDir)
	if err != nil {
		return err
	}
	a.metricsStore = metricsStore

	// Wire token tracker to metrics store
	a.tokenTracker.SetPersistFn(func(record *token.UsageRecord) error {
		return a.metricsStore.Save(record)
	})

	// Initialize vector store
	vectorStore, err := vector.NewMemoryStore(a.config.DataDir, 0)
	if err != nil {
		return err
	}
	a.vectorStore = vectorStore

	// Initialize embedding service
	embedConfig := embedding.DefaultConfig()
	embedConfig.Provider = embedding.ProviderMock // Use mock by default
	a.embedService = embedding.NewService(embedConfig)
	a.embedService.SetTokenTracker(a.tokenTracker)

	// Wire embedding function to vector store
	a.vectorStore.(*vector.MemoryStore).SetEmbeddingFunction(func(text string) ([]float32, error) {
		return a.embedService.Embed(context.Background(), text)
	})

	// Initialize agent registry
	a.agentRegistry = agent.NewRegistry(a.ctx)

	// Initialize global cluster services (Interest Manager & Event Router)
	a.interestMgr = interest.NewManager()
	a.eventRouter = event.NewRouter(a.interestMgr, &event.RouterConfig{
		NodeID:      nodeID,
		NodeName:    nodeName,
		VectorStore: vector.NewPortsAdapter(a.vectorStore),
	})

	// Create event bridge for P2P integration
	a.eventBridge = libp2p.NewEventBridge(a.node, a.eventRouter)
	a.eventBridge.SetInterestManager(a.interestMgr)

	// Register interests from environment variable
	a.registerInterestsFromEnv(nodeID, nodeName)

	return nil
}

// registerInterestsFromEnv registers interests from AGENT_COLLAB_INTERESTS environment variable.
func (a *App) registerInterestsFromEnv(nodeID, nodeName string) {
	if a.interestMgr == nil {
		return
	}

	// Get agent name from environment or use provided name
	agentName := os.Getenv("AGENT_NAME")
	if agentName == "" {
		agentName = nodeName
	}

	// Register interests from environment
	registered, err := interest.RegisterFromEnvironment(a.interestMgr, nodeID, agentName)
	if err != nil {
		if a.logger != nil {
			a.logger.Warn("Failed to register interests from environment", "error", err)
		}
		return
	}

	if registered != nil {
		if a.logger != nil {
			a.logger.Info("Registered interests from environment",
				"agent_id", nodeID,
				"agent_name", agentName,
				"patterns", registered.Patterns,
				"level", registered.Level.String())
		}
	}
}

// PublishContextSharedEvent publishes a context shared event to EventRouter.
// This is the single source of truth for publishing context events.
func (a *App) PublishContextSharedEvent(ctx context.Context, filePath, content string, embedding []float32) {
	if a.eventRouter == nil {
		return
	}

	nodeID := ""
	nodeName := os.Getenv("AGENT_NAME")
	if a.node != nil {
		nodeID = a.node.ID().String()
		if nodeName == "" {
			nodeName = "Agent-" + nodeID[:8]
		}
	}
	if nodeName == "" {
		nodeName = "Agent"
	}

	evt := event.NewContextSharedEvent(nodeID, nodeName, filePath, &event.ContextSharedPayload{
		Content: content,
	})
	evt.Embedding = embedding

	_ = a.eventRouter.Publish(ctx, evt)
}

// CreateInviteToken creates an invite token.
func (a *App) CreateInviteToken() (string, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()

	if a.node == nil {
		return "", fmt.Errorf("app is not initialized")
	}

	addrs := a.node.Addrs()
	addrStrs := make([]string, len(addrs))
	for i, addr := range addrs {
		// 전체 주소 (peer ID 포함)
		fullAddr := addr.String() + "/p2p/" + a.node.ID().String()
		addrStrs[i] = fullAddr
	}

	token, err := crypto.NewInviteToken(addrStrs, a.config.ProjectName, a.node.ID().String())
	if err != nil {
		return "", err
	}

	return token.Encode()
}
