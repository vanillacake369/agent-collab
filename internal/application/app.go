package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"agent-collab/internal/domain/agent"
	"agent-collab/internal/domain/ctxsync"
	"agent-collab/internal/domain/lock"
	"agent-collab/internal/domain/token"
	"agent-collab/internal/infrastructure/crypto"
	"agent-collab/internal/infrastructure/embedding"
	"agent-collab/internal/infrastructure/network/libp2p"
	"agent-collab/internal/infrastructure/storage/metrics"
	"agent-collab/internal/infrastructure/storage/vector"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// App is the main application orchestrator.
type App struct {
	mu sync.RWMutex

	// Configuration
	config *Config

	// Infrastructure
	keyPair      *crypto.KeyPair
	node         *libp2p.Node
	vectorStore  vector.Store
	metricsStore *metrics.Store
	embedService *embedding.Service

	// Domain services
	lockService   *lock.LockService
	syncManager   *ctxsync.SyncManager
	tokenTracker  *token.Tracker
	agentRegistry *agent.Registry

	// State
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// Config는 애플리케이션 설정입니다.
type Config struct {
	ProjectName string   `json:"project_name"`
	DataDir     string   `json:"data_dir"`
	ListenPort  int      `json:"listen_port"`
	Bootstrap   []string `json:"bootstrap"`
}

// DefaultConfig는 기본 설정을 반환합니다.
func DefaultConfig() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		DataDir:    filepath.Join(home, ".agent-collab"),
		ListenPort: 0, // 자동 할당
		Bootstrap:  []string{},
	}
}

// New는 새 애플리케이션을 생성합니다.
func New(cfg *Config) (*App, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 데이터 디렉토리 생성 (0700 for security - contains keys)
	if err := os.MkdirAll(cfg.DataDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create data dir: %w", err)
	}

	return &App{
		config: cfg,
	}, nil
}

// Initialize는 클러스터를 초기화합니다.
func (a *App) Initialize(ctx context.Context, projectName string) (*InitResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return nil, fmt.Errorf("app is already running")
	}

	a.config.ProjectName = projectName

	// 1. 키 생성
	keyPair, err := crypto.GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate keys: %w", err)
	}
	a.keyPair = keyPair

	// 키 저장
	keyPath := filepath.Join(a.config.DataDir, "key.json")
	if err := crypto.SaveKeyPair(keyPair, keyPath); err != nil {
		return nil, fmt.Errorf("failed to save keys: %w", err)
	}

	// 2. libp2p 노드 생성
	nodeConfig := libp2p.DefaultConfig()
	nodeConfig.PrivateKey = keyPair.PrivateKey
	nodeConfig.ProjectID = projectName

	node, err := libp2p.NewNode(ctx, nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}
	a.node = node

	// 3. Initialize domain services
	nodeIDStr := a.node.ID().String()
	a.lockService = lock.NewLockService(ctx, nodeIDStr, projectName+"-agent")
	a.syncManager = ctxsync.NewSyncManager(nodeIDStr, projectName+"-agent")

	// 4. Initialize Phase 3 components
	if err := a.initPhase3Components(nodeIDStr, projectName+"-agent"); err != nil {
		return nil, fmt.Errorf("failed to initialize Phase 3 components: %w", err)
	}

	// 5. Build address string list
	addrs := a.node.Addrs()
	addrStrs := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrs[i] = addr.String()
	}

	// 6. Create invite token
	token, err := crypto.NewInviteToken(addrStrs, projectName, nodeIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to create invite token: %w", err)
	}

	tokenStr, err := token.Encode()
	if err != nil {
		return nil, fmt.Errorf("failed to encode invite token: %w", err)
	}

	// Save config for daemon to load later
	if err := a.saveConfig(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	return &InitResult{
		ProjectName: projectName,
		NodeID:      nodeIDStr,
		Addresses:   addrStrs,
		InviteToken: tokenStr,
		KeyPath:     keyPath,
	}, nil
}

// saveConfig saves the app configuration to disk.
func (a *App) saveConfig() error {
	configPath := filepath.Join(a.config.DataDir, "config.json")
	data, err := json.MarshalIndent(a.config, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0600)
}

// LoadFromConfig loads an existing configuration and initializes the app.
func (a *App) LoadFromConfig(ctx context.Context) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("app is already running")
	}

	// Load config
	configPath := filepath.Join(a.config.DataDir, "config.json")
	// #nosec G304 - configPath is constructed from app's DataDir, not user input
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("config not found (run 'init' first): %w", err)
	}

	if err := json.Unmarshal(data, a.config); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	// Set context
	a.ctx, a.cancel = context.WithCancel(ctx)

	// Load keys
	keyPath := filepath.Join(a.config.DataDir, "key.json")
	keyPair, err := crypto.LoadKeyPair(keyPath)
	if err != nil {
		return fmt.Errorf("failed to load keys: %w", err)
	}
	a.keyPair = keyPair

	// Create libp2p node
	nodeConfig := libp2p.DefaultConfig()
	nodeConfig.PrivateKey = keyPair.PrivateKey
	nodeConfig.ProjectID = a.config.ProjectName

	node, err := libp2p.NewNode(ctx, nodeConfig)
	if err != nil {
		return fmt.Errorf("failed to create node: %w", err)
	}
	a.node = node

	// Initialize domain services
	nodeIDStr := a.node.ID().String()
	agentID := a.config.ProjectName + "-agent"
	a.lockService = lock.NewLockService(ctx, nodeIDStr, agentID)
	a.syncManager = ctxsync.NewSyncManager(nodeIDStr, agentID)

	// Initialize Phase 3 components
	if err := a.initPhase3Components(nodeIDStr, agentID); err != nil {
		return fmt.Errorf("failed to initialize components: %w", err)
	}

	return nil
}

// Join은 클러스터에 참여합니다.
func (a *App) Join(ctx context.Context, tokenStr string) (*JoinResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return nil, fmt.Errorf("app is already running")
	}

	// 1. Decode and validate token
	token, err := crypto.DecodeInviteToken(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid invite token: %w", err)
	}

	// Check token expiration
	if token.IsExpired() {
		return nil, fmt.Errorf("invite token has expired")
	}

	a.config.ProjectName = token.ProjectName

	// 2. 키 생성 또는 로드
	keyPath := filepath.Join(a.config.DataDir, "key.json")
	keyPair, err := crypto.LoadKeyPair(keyPath)
	if err != nil {
		keyPair, err = crypto.GenerateKeyPair()
		if err != nil {
			return nil, fmt.Errorf("failed to generate keys: %w", err)
		}
		if err := crypto.SaveKeyPair(keyPair, keyPath); err != nil {
			return nil, fmt.Errorf("failed to save keys: %w", err)
		}
	}
	a.keyPair = keyPair

	// 3. Bootstrap peer 주소 파싱
	var bootstrapPeers []peer.AddrInfo
	for _, addrStr := range token.Addresses {
		ma, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			continue
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			// 주소에 peer ID가 없으면 토큰의 CreatorID 사용
			creatorID, err := peer.Decode(token.CreatorID)
			if err != nil {
				continue
			}
			peerInfo = &peer.AddrInfo{
				ID:    creatorID,
				Addrs: []multiaddr.Multiaddr{ma},
			}
		}
		bootstrapPeers = append(bootstrapPeers, *peerInfo)
	}

	// 4. libp2p 노드 생성
	nodeConfig := libp2p.DefaultConfig()
	nodeConfig.PrivateKey = keyPair.PrivateKey
	nodeConfig.ProjectID = token.ProjectName
	nodeConfig.BootstrapPeers = bootstrapPeers

	node, err := libp2p.NewNode(ctx, nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}
	a.node = node

	// 5. Initialize domain services
	nodeIDStr := a.node.ID().String()
	a.lockService = lock.NewLockService(ctx, nodeIDStr, token.ProjectName+"-agent")
	a.syncManager = ctxsync.NewSyncManager(nodeIDStr, token.ProjectName+"-agent")

	// 6. Initialize Phase 3 components
	if err := a.initPhase3Components(nodeIDStr, token.ProjectName+"-agent"); err != nil {
		return nil, fmt.Errorf("failed to initialize Phase 3 components: %w", err)
	}

	// 7. Perform bootstrap
	if err := a.node.Bootstrap(ctx, bootstrapPeers); err != nil {
		fmt.Printf("Bootstrap warning: %v\n", err)
	}

	return &JoinResult{
		ProjectName:    token.ProjectName,
		NodeID:         nodeIDStr,
		BootstrapPeer:  token.CreatorID,
		ConnectedPeers: len(a.node.ConnectedPeers()),
	}, nil
}

// Start는 애플리케이션을 시작합니다.
func (a *App) Start() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return fmt.Errorf("app is already running")
	}

	if a.node == nil {
		return fmt.Errorf("app is not initialized")
	}

	ctx, cancel := context.WithCancel(context.Background())
	a.ctx = ctx
	a.cancel = cancel
	a.running = true

	// 동기화 관리자 시작
	a.syncManager.Start(ctx)

	// 메시지 핸들러 설정
	a.setupMessageHandlers()

	// 프로젝트 토픽 구독
	if err := a.node.SubscribeProjectTopics(ctx); err != nil {
		return fmt.Errorf("failed to subscribe topics: %w", err)
	}

	// Start message processing goroutines
	go a.processLockMessages(ctx)
	go a.processContextMessages(ctx)

	return nil
}

// Stop stops the application.
func (a *App) Stop() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if !a.running {
		return nil
	}

	if a.cancel != nil {
		a.cancel()
	}

	if a.lockService != nil {
		a.lockService.Close()
	}

	if a.syncManager != nil {
		a.syncManager.Stop()
	}

	// Close Phase 3 components
	if a.tokenTracker != nil {
		a.tokenTracker.Close()
	}

	if a.metricsStore != nil {
		a.metricsStore.Close()
	}

	if a.vectorStore != nil {
		a.vectorStore.Close()
	}

	if a.node != nil {
		a.node.Close()
	}

	a.running = false
	return nil
}

// setupMessageHandlers는 메시지 핸들러를 설정합니다.
func (a *App) setupMessageHandlers() {
	// 락 서비스 브로드캐스트 설정
	a.lockService.SetBroadcastFn(func(msg any) error {
		data, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		topicName := "/agent-collab/" + a.config.ProjectName + "/lock"
		return a.node.Publish(a.ctx, topicName, data)
	})

	// 동기화 관리자 브로드캐스트 설정
	a.syncManager.SetBroadcastFn(func(delta *ctxsync.Delta) error {
		data, err := json.Marshal(delta)
		if err != nil {
			return err
		}
		topicName := "/agent-collab/" + a.config.ProjectName + "/context"
		return a.node.Publish(a.ctx, topicName, data)
	})

	// 충돌 핸들러 설정
	a.lockService.SetConflictHandler(func(conflict *lock.LockConflict) error {
		fmt.Printf("Lock conflict detected: %s vs %s\n",
			conflict.RequestedLock.HolderName,
			conflict.ConflictingLock.HolderName)
		return nil
	})

	a.syncManager.SetConflictHandler(func(conflict *ctxsync.Conflict) error {
		fmt.Printf("Concurrent modification conflict: %s\n", conflict.FilePath)
		return nil
	})
}

// LockMessage is a message for lock operations.
type LockMessage struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// processLockMessages processes incoming lock messages from P2P network.
func (a *App) processLockMessages(ctx context.Context) {
	topicName := "/agent-collab/" + a.config.ProjectName + "/lock"
	sub := a.node.GetSubscription(topicName)
	if sub == nil {
		fmt.Printf("No subscription for topic: %s\n", topicName)
		return
	}

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, graceful shutdown
			}
			fmt.Printf("Error receiving lock message: %v\n", err)
			continue
		}

		// Skip messages from ourselves
		if msg.ReceivedFrom == a.node.ID() {
			continue
		}

		var lockMsg LockMessage
		if err := json.Unmarshal(msg.Data, &lockMsg); err != nil {
			fmt.Printf("Error unmarshaling lock message: %v\n", err)
			continue
		}

		switch lockMsg.Type {
		case "intent":
			var intent lock.LockIntent
			if err := json.Unmarshal(lockMsg.Data, &intent); err != nil {
				fmt.Printf("Error unmarshaling lock intent: %v\n", err)
				continue
			}
			if err := a.lockService.HandleRemoteLockIntent(&intent); err != nil {
				fmt.Printf("Error handling lock intent: %v\n", err)
			}

		case "acquired":
			var semanticLock lock.SemanticLock
			if err := json.Unmarshal(lockMsg.Data, &semanticLock); err != nil {
				fmt.Printf("Error unmarshaling acquired lock: %v\n", err)
				continue
			}
			if err := a.lockService.HandleRemoteLockAcquired(&semanticLock); err != nil {
				fmt.Printf("Error handling lock acquired: %v\n", err)
			}

		case "released":
			var releaseMsg struct {
				LockID string `json:"lock_id"`
			}
			if err := json.Unmarshal(lockMsg.Data, &releaseMsg); err != nil {
				fmt.Printf("Error unmarshaling lock release: %v\n", err)
				continue
			}
			if err := a.lockService.HandleRemoteLockReleased(releaseMsg.LockID); err != nil {
				fmt.Printf("Error handling lock released: %v\n", err)
			}
		}
	}
}

// processContextMessages processes incoming context sync messages from P2P network.
func (a *App) processContextMessages(ctx context.Context) {
	topicName := "/agent-collab/" + a.config.ProjectName + "/context"
	sub := a.node.GetSubscription(topicName)
	if sub == nil {
		fmt.Printf("No subscription for topic: %s\n", topicName)
		return
	}

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, graceful shutdown
			}
			fmt.Printf("Error receiving context message: %v\n", err)
			continue
		}

		// Skip messages from ourselves
		if msg.ReceivedFrom == a.node.ID() {
			continue
		}

		var delta ctxsync.Delta
		if err := json.Unmarshal(msg.Data, &delta); err != nil {
			fmt.Printf("Error unmarshaling delta: %v\n", err)
			continue
		}

		if err := a.syncManager.ReceiveDelta(&delta); err != nil {
			fmt.Printf("Error handling delta: %v\n", err)
		}
	}
}

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

// AgentRegistry returns the agent registry.
func (a *App) AgentRegistry() *agent.Registry {
	return a.agentRegistry
}

// initPhase3Components initializes token tracking, vector storage, and embedding.
func (a *App) initPhase3Components(nodeID, nodeName string) error {
	// Initialize token tracker
	a.tokenTracker = token.NewTracker(nodeID, nodeName)

	// Initialize metrics store
	metricsStore, err := metrics.NewStore(a.config.DataDir)
	if err != nil {
		return fmt.Errorf("failed to create metrics store: %w", err)
	}
	a.metricsStore = metricsStore

	// Wire token tracker to metrics store
	a.tokenTracker.SetPersistFn(func(record *token.UsageRecord) error {
		return a.metricsStore.Save(record)
	})

	// Initialize vector store
	vectorStore, err := vector.NewMemoryStore(a.config.DataDir, 0)
	if err != nil {
		return fmt.Errorf("failed to create vector store: %w", err)
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

	return nil
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

// InitResult는 초기화 결과입니다.
type InitResult struct {
	ProjectName string   `json:"project_name"`
	NodeID      string   `json:"node_id"`
	Addresses   []string `json:"addresses"`
	InviteToken string   `json:"invite_token"`
	KeyPath     string   `json:"key_path"`
}

// JoinResult는 참여 결과입니다.
type JoinResult struct {
	ProjectName    string `json:"project_name"`
	NodeID         string `json:"node_id"`
	BootstrapPeer  string `json:"bootstrap_peer"`
	ConnectedPeers int    `json:"connected_peers"`
}

// Status holds the application status.
type Status struct {
	Running      bool     `json:"running"`
	ProjectName  string   `json:"project_name"`
	NodeID       string   `json:"node_id"`
	Addresses    []string `json:"addresses"`
	PeerCount    int      `json:"peer_count"`
	LockCount    int      `json:"lock_count"`
	MyLockCount  int      `json:"my_lock_count"`
	DeltaCount   int      `json:"delta_count"`
	WatchedFiles int      `json:"watched_files"`

	// Token usage (Phase 3)
	TokensToday   int64   `json:"tokens_today"`
	TokensPerHour float64 `json:"tokens_per_hour"`
	CostToday     float64 `json:"cost_today"`

	// Vector store (Phase 3)
	EmbeddingCount int64 `json:"embedding_count"`
}

// Ensure libp2pcrypto is used
var _ libp2pcrypto.PrivKey = nil
