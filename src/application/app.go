package application

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

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
	"agent-collab/src/pkg/logging"

	libp2pcrypto "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// App is the main application orchestrator.
type App struct {
	mu sync.RWMutex

	// Logging
	logger *logging.Logger

	// Configuration
	config *Config

	// Infrastructure
	keyPair      *crypto.KeyPair
	node         *libp2p.Node
	vectorStore  vector.Store
	metricsStore *metrics.Store
	embedService *embedding.Service

	// WireGuard VPN (optional)
	wgManager *wireguard.WireGuardManager

	// Domain services
	lockService   *lock.LockService
	syncManager   *ctxsync.SyncManager
	tokenTracker  *token.Tracker
	agentRegistry *agent.Registry

	// Global cluster services
	interestMgr *interest.Manager
	eventRouter *event.Router
	eventBridge *libp2p.EventBridge

	// State
	running bool
	ctx     context.Context
	cancel  context.CancelFunc
}

// Config는 애플리케이션 설정입니다.
type Config struct {
	ProjectName   string   `json:"project_name"`
	DataDir       string   `json:"data_dir"`
	ListenPort    int      `json:"listen_port"`
	ListenAddrs   []string `json:"listen_addrs,omitempty"`   // 실제 바인딩된 주소들
	Bootstrap     []string `json:"bootstrap"`                // Bootstrap peer 주소들
	BootstrapPeer string   `json:"bootstrap_peer,omitempty"` // Bootstrap peer ID

	// WireGuard VPN settings
	WireGuard *WireGuardConfig `json:"wireguard,omitempty"`
}

// WireGuardConfig holds WireGuard VPN configuration.
type WireGuardConfig struct {
	Enabled             bool   `json:"enabled"`
	ListenPort          int    `json:"listen_port"`
	Subnet              string `json:"subnet"`
	MTU                 int    `json:"mtu"`
	PersistentKeepalive int    `json:"persistent_keepalive"`
	InterfaceName       string `json:"interface_name"`
}

// DefaultWireGuardConfig returns default WireGuard configuration.
func DefaultWireGuardConfig() *WireGuardConfig {
	return &WireGuardConfig{
		Enabled:             false,
		ListenPort:          51820,
		Subnet:              "10.100.0.0/24",
		MTU:                 1420,
		PersistentKeepalive: 25,
		InterfaceName:       "wg-agent",
	}
}

// DefaultConfig는 기본 설정을 반환합니다.
func DefaultConfig() *Config {
	// Check environment variable first (for Docker/container use)
	dataDir := os.Getenv("AGENT_COLLAB_DATA_DIR")
	if dataDir == "" {
		home, _ := os.UserHomeDir()
		dataDir = filepath.Join(home, ".agent-collab")
	}

	return &Config{
		DataDir:    dataDir,
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

	// Initialize structured logger
	logger := logging.New(os.Stdout, "info").Component("app")

	return &App{
		config: cfg,
		logger: logger,
	}, nil
}

// InitializeOptions holds options for cluster initialization.
type InitializeOptions struct {
	ProjectName     string
	EnableWireGuard bool
	WireGuardPort   int
	Subnet          string
}

// Initialize는 클러스터를 초기화합니다.
func (a *App) Initialize(ctx context.Context, projectName string) (*InitResult, error) {
	return a.InitializeWithOptions(ctx, &InitializeOptions{ProjectName: projectName})
}

// InitializeWithOptions initializes the cluster with options.
func (a *App) InitializeWithOptions(ctx context.Context, opts *InitializeOptions) (*InitResult, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.running {
		return nil, fmt.Errorf("app is already running")
	}

	// Set context
	a.ctx, a.cancel = context.WithCancel(ctx)

	a.config.ProjectName = opts.ProjectName

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

	// 2. Initialize WireGuard if enabled
	var wgInfo *crypto.WireGuardInfo
	if opts.EnableWireGuard {
		wgInfo, err = a.initializeWireGuard(ctx, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize WireGuard: %w", err)
		}
	}

	// 3. libp2p 노드 생성 (global cluster - no projectID)
	nodeConfig := libp2p.DefaultConfig()
	nodeConfig.PrivateKey = keyPair.PrivateKey

	node, err := libp2p.NewNode(ctx, nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}
	a.node = node

	// 4. Initialize domain services
	nodeIDStr := a.node.ID().String()
	a.lockService = lock.NewLockService(ctx, nodeIDStr, opts.ProjectName+"-agent")
	a.syncManager = ctxsync.NewSyncManager(nodeIDStr, opts.ProjectName+"-agent")

	// 5. Initialize Phase 3 components
	if err := a.initPhase3Components(nodeIDStr, opts.ProjectName+"-agent"); err != nil {
		return nil, fmt.Errorf("failed to initialize Phase 3 components: %w", err)
	}

	// 6. Build address string list and save to config
	addrs := a.node.Addrs()
	addrStrs := make([]string, len(addrs))
	for i, addr := range addrs {
		addrStrs[i] = addr.String()
	}
	// Save the actual listen addresses for daemon restart
	a.config.ListenAddrs = addrStrs

	// 7. Create invite token
	var tokenStr string
	if wgInfo != nil {
		// Create WireGuard-enabled token
		token, err := crypto.NewWireGuardToken(addrStrs, opts.ProjectName, nodeIDStr, wgInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to create wireguard token: %w", err)
		}
		tokenStr, err = token.Encode()
		if err != nil {
			return nil, fmt.Errorf("failed to encode wireguard token: %w", err)
		}
	} else {
		// Create simple token
		token, err := crypto.NewInviteToken(addrStrs, opts.ProjectName, nodeIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to create invite token: %w", err)
		}
		tokenStr, err = token.Encode()
		if err != nil {
			return nil, fmt.Errorf("failed to encode invite token: %w", err)
		}
	}

	// Save config for daemon to load later
	if err := a.saveConfig(); err != nil {
		return nil, fmt.Errorf("failed to save config: %w", err)
	}

	result := &InitResult{
		ProjectName: opts.ProjectName,
		NodeID:      nodeIDStr,
		Addresses:   addrStrs,
		InviteToken: tokenStr,
		KeyPath:     keyPath,
	}

	// Add WireGuard info to result
	if a.wgManager != nil {
		result.WireGuardEnabled = true
		result.WireGuardIP = a.wgManager.GetLocalIP()
		result.WireGuardEndpoint = a.wgManager.GetEndpoint()
	}

	return result, nil
}

// initializeWireGuard initializes the WireGuard VPN manager.
func (a *App) initializeWireGuard(ctx context.Context, opts *InitializeOptions) (*crypto.WireGuardInfo, error) {
	// Set up WireGuard config
	if a.config.WireGuard == nil {
		a.config.WireGuard = DefaultWireGuardConfig()
	}
	a.config.WireGuard.Enabled = true

	if opts.WireGuardPort > 0 {
		a.config.WireGuard.ListenPort = opts.WireGuardPort
	}
	if opts.Subnet != "" {
		a.config.WireGuard.Subnet = opts.Subnet
	}

	// Create manager
	mgr := wireguard.NewManager(nil)
	mgrCfg := &wireguard.ManagerConfig{
		InterfaceName:       a.config.WireGuard.InterfaceName,
		ListenPort:          a.config.WireGuard.ListenPort,
		Subnet:              a.config.WireGuard.Subnet,
		MTU:                 a.config.WireGuard.MTU,
		PersistentKeepalive: a.config.WireGuard.PersistentKeepalive,
		AutoDetectEndpoint:  true,
	}

	if err := mgr.Initialize(ctx, mgrCfg); err != nil {
		return nil, fmt.Errorf("failed to initialize WireGuard manager: %w", err)
	}

	// Start the WireGuard interface
	if err := mgr.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start WireGuard: %w", err)
	}

	a.wgManager = mgr

	// Save WireGuard config
	wgConfigPath := filepath.Join(a.config.DataDir, "wireguard.json")
	if err := wireguard.SaveConfigFile(mgr.GetConfig().ToConfigFile(), wgConfigPath); err != nil {
		return nil, fmt.Errorf("failed to save WireGuard config: %w", err)
	}

	// Build WireGuard info for token
	keyPair := mgr.GetKeyPair()
	return &crypto.WireGuardInfo{
		CreatorPublicKey: keyPair.PublicKey,
		CreatorEndpoint:  mgr.GetEndpoint(),
		Subnet:           a.config.WireGuard.Subnet,
		CreatorIP:        mgr.GetLocalIP(),
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

	// Create libp2p node with saved listen addresses (global cluster - no projectID)
	nodeConfig := libp2p.DefaultConfig()
	nodeConfig.PrivateKey = keyPair.PrivateKey

	// Use saved listen addresses if available (to keep same ports)
	if len(a.config.ListenAddrs) > 0 {
		nodeConfig.ListenAddrs = a.config.ListenAddrs
	}

	// Load bootstrap peers if configured
	if len(a.config.Bootstrap) > 0 {
		for _, addrStr := range a.config.Bootstrap {
			ma, err := multiaddr.NewMultiaddr(addrStr)
			if err != nil {
				continue
			}
			peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
			if err != nil {
				continue
			}
			nodeConfig.BootstrapPeers = append(nodeConfig.BootstrapPeers, *peerInfo)
		}
	}

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

	// Set context
	a.ctx, a.cancel = context.WithCancel(ctx)

	// 1. Decode and validate token (try WireGuard token first)
	token, hasWireGuard, err := crypto.DecodeAnyToken(tokenStr)
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

	// 3. Initialize WireGuard if token has WireGuard info
	if hasWireGuard && token.WireGuard != nil {
		if err := a.joinWithWireGuard(ctx, token.WireGuard); err != nil {
			// Log warning but continue with libp2p-only mode
			a.logger.Warn("WireGuard setup failed, using libp2p only", "error", err)
		}
	}

	// 4. Bootstrap peer 주소 파싱
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

	// 5. libp2p 노드 생성 (global cluster - no projectID)
	nodeConfig := libp2p.DefaultConfig()
	nodeConfig.PrivateKey = keyPair.PrivateKey
	nodeConfig.BootstrapPeers = bootstrapPeers

	node, err := libp2p.NewNode(ctx, nodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create node: %w", err)
	}
	a.node = node

	// 6. Initialize domain services
	nodeIDStr := a.node.ID().String()
	a.lockService = lock.NewLockService(ctx, nodeIDStr, token.ProjectName+"-agent")
	a.syncManager = ctxsync.NewSyncManager(nodeIDStr, token.ProjectName+"-agent")

	// 7. Initialize Phase 3 components
	if err := a.initPhase3Components(nodeIDStr, token.ProjectName+"-agent"); err != nil {
		return nil, fmt.Errorf("failed to initialize Phase 3 components: %w", err)
	}

	// 8. Perform bootstrap
	if err := a.node.Bootstrap(ctx, bootstrapPeers); err != nil {
		a.logger.Warn("bootstrap encountered issues", "error", err)
	}

	// 9. Save listen addresses and bootstrap info to config
	addrs := a.node.Addrs()
	a.config.ListenAddrs = make([]string, len(addrs))
	for i, addr := range addrs {
		a.config.ListenAddrs[i] = addr.String()
	}
	a.config.Bootstrap = token.Addresses
	a.config.BootstrapPeer = token.CreatorID

	// Save config for daemon to load later
	if err := a.saveConfig(); err != nil {
		a.logger.Warn("failed to save config", "error", err)
	}

	result := &JoinResult{
		ProjectName:    token.ProjectName,
		NodeID:         nodeIDStr,
		BootstrapPeer:  token.CreatorID,
		ConnectedPeers: len(a.node.ConnectedPeers()),
	}

	// Add WireGuard info to result
	if a.wgManager != nil {
		result.WireGuardEnabled = true
		result.WireGuardIP = a.wgManager.GetLocalIP()
	}

	return result, nil
}

// joinWithWireGuard sets up WireGuard VPN connection to the cluster.
func (a *App) joinWithWireGuard(ctx context.Context, wgInfo *crypto.WireGuardInfo) error {
	// Set up WireGuard config
	if a.config.WireGuard == nil {
		a.config.WireGuard = DefaultWireGuardConfig()
	}
	a.config.WireGuard.Enabled = true
	a.config.WireGuard.Subnet = wgInfo.Subnet

	// Create manager
	mgr := wireguard.NewManager(nil)
	mgrCfg := &wireguard.ManagerConfig{
		InterfaceName:       a.config.WireGuard.InterfaceName,
		ListenPort:          a.config.WireGuard.ListenPort,
		Subnet:              wgInfo.Subnet,
		MTU:                 a.config.WireGuard.MTU,
		PersistentKeepalive: a.config.WireGuard.PersistentKeepalive,
		AutoDetectEndpoint:  true,
	}

	if err := mgr.Initialize(ctx, mgrCfg); err != nil {
		return fmt.Errorf("failed to initialize WireGuard manager: %w", err)
	}

	// Add creator as peer
	creatorPeer := &wireguard.Peer{
		PublicKey:           wgInfo.CreatorPublicKey,
		Endpoint:            wgInfo.CreatorEndpoint,
		AllowedIPs:          []string{wgInfo.CreatorIP},
		PersistentKeepalive: a.config.WireGuard.PersistentKeepalive,
	}
	if err := mgr.AddPeer(creatorPeer); err != nil {
		return fmt.Errorf("failed to add creator peer: %w", err)
	}

	// Start the WireGuard interface
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start WireGuard: %w", err)
	}

	a.wgManager = mgr

	// Save WireGuard config
	wgConfigPath := filepath.Join(a.config.DataDir, "wireguard.json")
	if err := wireguard.SaveConfigFile(mgr.GetConfig().ToConfigFile(), wgConfigPath); err != nil {
		a.logger.Warn("failed to save WireGuard config", "error", err)
	}

	return nil
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

	// Bootstrap to peers if configured
	if len(a.config.Bootstrap) > 0 && a.config.BootstrapPeer != "" {
		// Parse bootstrap peer ID
		bootstrapPeerID, err := peer.Decode(a.config.BootstrapPeer)
		if err == nil {
			var bootstrapAddrs []multiaddr.Multiaddr
			for _, addrStr := range a.config.Bootstrap {
				ma, err := multiaddr.NewMultiaddr(addrStr)
				if err != nil {
					continue
				}
				bootstrapAddrs = append(bootstrapAddrs, ma)
			}
			if len(bootstrapAddrs) > 0 {
				bootstrapPeers := []peer.AddrInfo{{
					ID:    bootstrapPeerID,
					Addrs: bootstrapAddrs,
				}}
				go func() {
					if err := a.node.Bootstrap(ctx, bootstrapPeers); err != nil {
						a.logger.Warn("bootstrap encountered issues", "error", err)
					}
				}()
			}
		}
	}

	// 동기화 관리자 시작
	a.syncManager.Start(ctx)

	// 메시지 핸들러 설정
	a.setupMessageHandlers()

	// 글로벌 토픽 구독
	if err := a.node.SubscribeGlobalTopics(ctx); err != nil {
		return fmt.Errorf("failed to subscribe topics: %w", err)
	}

	// Start event bridge for P2P event routing
	if a.eventBridge != nil {
		if err := a.eventBridge.Start(ctx); err != nil {
			return fmt.Errorf("failed to start event bridge: %w", err)
		}
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

	// Stop event bridge
	if a.eventBridge != nil {
		a.eventBridge.Stop()
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

	// Stop WireGuard VPN
	if a.wgManager != nil {
		if err := a.wgManager.Stop(); err != nil {
			a.logger.Warn("failed to stop WireGuard", "error", err)
		}
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
	conflictLog := a.logger.Component("conflict")
	a.lockService.SetConflictHandler(func(conflict *lock.LockConflict) error {
		conflictLog.Warn("lock conflict detected",
			"requested_by", conflict.RequestedLock.HolderName,
			"conflicting_with", conflict.ConflictingLock.HolderName,
			"overlap_type", conflict.OverlapType)
		return nil
	})

	a.syncManager.SetConflictHandler(func(conflict *ctxsync.Conflict) error {
		conflictLog.Warn("concurrent modification conflict", "file_path", conflict.FilePath)
		return nil
	})
}

// LockMessageBase is a base type for determining message type.
type LockMessageBase struct {
	Type string `json:"type"`
}

// IntentMessageWrapper matches the format from lock.IntentMessage.
type IntentMessageWrapper struct {
	Type   string           `json:"type"`
	Intent *lock.LockIntent `json:"intent"`
}

// AcquireMessageWrapper matches the format from lock.AcquireMessage.
type AcquireMessageWrapper struct {
	Type string             `json:"type"`
	Lock *lock.SemanticLock `json:"lock"`
}

// ReleaseMessageWrapper matches the format from lock.ReleaseMessage.
type ReleaseMessageWrapper struct {
	Type   string `json:"type"`
	LockID string `json:"lock_id"`
}

// processLockMessages processes incoming lock messages from P2P network.
func (a *App) processLockMessages(ctx context.Context) {
	log := a.logger.Component("lock-processor")
	topicName := "/agent-collab/" + a.config.ProjectName + "/lock"
	sub := a.node.GetSubscription(topicName)
	if sub == nil {
		log.Warn("no subscription for topic", "topic", topicName)
		return
	}

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, graceful shutdown
			}
			log.Error("failed to receive lock message", "error", err)
			continue
		}

		// Skip messages from ourselves
		if msg.ReceivedFrom == a.node.ID() {
			continue
		}

		// Decompress message if needed
		data, err := libp2p.DecompressMessage(msg.Data)
		if err != nil {
			// Try raw data for backward compatibility
			data = msg.Data
		}

		// Handle batch or single message
		messages, err := libp2p.UnbatchMessage(data)
		if err != nil {
			log.Error("failed to unbatch lock message", "error", err)
			continue
		}

		for _, msgData := range messages {
			a.handleSingleLockMessage(msgData)
		}
	}
}

// handleSingleLockMessage processes a single lock message
func (a *App) handleSingleLockMessage(data []byte) {
	log := a.logger.Component("lock-handler")

	var baseMsg LockMessageBase
	if err := json.Unmarshal(data, &baseMsg); err != nil {
		log.Error("failed to unmarshal lock message type", "error", err)
		return
	}

	switch baseMsg.Type {
	case "lock_intent":
		var intentMsg IntentMessageWrapper
		if err := json.Unmarshal(data, &intentMsg); err != nil {
			log.Error("failed to unmarshal lock intent", "error", err)
			return
		}
		if intentMsg.Intent == nil {
			log.Warn("received lock_intent with nil intent")
			return
		}
		if err := a.lockService.HandleRemoteLockIntent(intentMsg.Intent); err != nil {
			log.Error("failed to handle lock intent", "error", err)
		}

	case "lock_acquired":
		var acquireMsg AcquireMessageWrapper
		if err := json.Unmarshal(data, &acquireMsg); err != nil {
			log.Error("failed to unmarshal acquired lock", "error", err)
			return
		}
		if acquireMsg.Lock == nil {
			log.Warn("received lock_acquired with nil lock")
			return
		}
		if err := a.lockService.HandleRemoteLockAcquired(acquireMsg.Lock); err != nil {
			log.Error("failed to handle lock acquired", "error", err)
		}

	case "lock_released":
		var releaseMsg ReleaseMessageWrapper
		if err := json.Unmarshal(data, &releaseMsg); err != nil {
			log.Error("failed to unmarshal lock release", "error", err)
			return
		}
		if err := a.lockService.HandleRemoteLockReleased(releaseMsg.LockID); err != nil {
			log.Error("failed to handle lock released", "error", err)
		}

	default:
		log.Warn("unknown lock message type", "type", baseMsg.Type)
	}
}

// ContextMessageBase is used to determine the message type.
type ContextMessageBase struct {
	Type string `json:"type"`
}

// processContextMessages processes incoming context sync messages from P2P network.
func (a *App) processContextMessages(ctx context.Context) {
	log := a.logger.Component("context-processor")
	topicName := "/agent-collab/" + a.config.ProjectName + "/context"
	sub := a.node.GetSubscription(topicName)
	if sub == nil {
		log.Warn("no subscription for topic", "topic", topicName)
		return
	}

	for {
		msg, err := sub.Next(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return // Context cancelled, graceful shutdown
			}
			log.Error("failed to receive context message", "error", err)
			continue
		}

		// Skip messages from ourselves
		if msg.ReceivedFrom == a.node.ID() {
			continue
		}

		// Decompress message if needed
		data, err := libp2p.DecompressMessage(msg.Data)
		if err != nil {
			// Try raw data for backward compatibility
			data = msg.Data
		}

		// Handle batch or single message
		messages, err := libp2p.UnbatchMessage(data)
		if err != nil {
			log.Error("failed to unbatch context message", "error", err)
			continue
		}

		for _, msgData := range messages {
			a.handleSingleContextMessage(ctx, msgData)
		}
	}
}

// handleSingleContextMessage processes a single context message
func (a *App) handleSingleContextMessage(ctx context.Context, data []byte) {
	log := a.logger.Component("context-handler")

	var baseMsg ContextMessageBase
	if err := json.Unmarshal(data, &baseMsg); err != nil {
		log.Error("failed to unmarshal context message type", "error", err)
		return
	}

	switch baseMsg.Type {
	case "shared_context":
		// Handle shared context from peers
		var ctxMsg ContextMessage
		if err := json.Unmarshal(data, &ctxMsg); err != nil {
			log.Error("failed to unmarshal shared context", "error", err)
			return
		}
		a.handleSharedContext(ctx, &ctxMsg)

	default:
		// Assume it's a Delta message (for backward compatibility)
		var delta ctxsync.Delta
		if err := json.Unmarshal(data, &delta); err != nil {
			log.Error("failed to unmarshal delta", "error", err)
			return
		}

		if err := a.syncManager.ReceiveDelta(&delta); err != nil {
			log.Error("failed to handle delta", "error", err)
		}

		// Also store in VectorDB if it's a file change with content
		a.storeDeltaInVectorDB(ctx, &delta)
	}
}

// handleSharedContext processes shared context from a peer and stores it in VectorDB.
func (a *App) handleSharedContext(ctx context.Context, msg *ContextMessage) {
	log := a.logger.Component("context-handler")

	if a.vectorStore == nil {
		return
	}

	// Use provided embedding or generate new one
	embedding := msg.Embedding
	if len(embedding) == 0 && a.embedService != nil && msg.Content != "" {
		var err error
		embedding, err = a.embedService.Embed(ctx, msg.Content)
		if err != nil {
			log.Error("failed to generate embedding for shared context", "error", err)
			return
		}
	}

	// Create and store document
	doc := &vector.Document{
		Content:   msg.Content,
		Embedding: embedding,
		FilePath:  msg.FilePath,
		Metadata:  msg.Metadata,
	}
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]any)
	}
	doc.Metadata["source_id"] = msg.SourceID
	doc.Metadata["type"] = "shared_context"

	if err := a.vectorStore.Insert(doc); err != nil {
		log.Error("failed to store shared context in VectorDB", "error", err)
		return
	}

	// Async flush
	go func() {
		if err := a.vectorStore.Flush(); err != nil {
			log.Error("failed to flush VectorDB", "error", err)
		}
	}()

	log.Info("received shared context", "source_id", msg.SourceID, "file_path", msg.FilePath)
}

// storeDeltaInVectorDB stores delta content in VectorDB for search.
func (a *App) storeDeltaInVectorDB(ctx context.Context, delta *ctxsync.Delta) {
	log := a.logger.Component("vector-store")

	if a.vectorStore == nil || a.embedService == nil {
		return
	}

	// Only process file changes
	if delta.Type != ctxsync.DeltaFileChange || delta.Payload.FilePath == "" {
		return
	}

	// Build content description from delta info
	content := fmt.Sprintf("File change: %s from %s",
		delta.Payload.FilePath, delta.SourceName)

	// Add symbol info if available
	if delta.Payload.FileDiff != nil {
		for _, d := range delta.Payload.FileDiff.Diffs {
			if d.Symbol != nil {
				content += fmt.Sprintf("\n%s %s: %s",
					d.Type, d.Symbol.Type, d.Symbol.Name)
			}
		}
	}

	// Generate embedding
	embedding, err := a.embedService.Embed(ctx, content)
	if err != nil {
		log.Error("failed to generate embedding for delta", "error", err, "file_path", delta.Payload.FilePath)
		return
	}

	// Create and store document
	doc := &vector.Document{
		Content:   content,
		Embedding: embedding,
		FilePath:  delta.Payload.FilePath,
		Metadata: map[string]any{
			"source_id":   delta.SourceID,
			"source_name": delta.SourceName,
			"delta_id":    delta.ID,
			"timestamp":   delta.Timestamp,
			"type":        "delta_sync",
		},
	}

	if err := a.vectorStore.Insert(doc); err != nil {
		log.Error("failed to store delta in VectorDB", "error", err, "file_path", delta.Payload.FilePath)
		return
	}

	// Async flush to avoid blocking
	go func() {
		if err := a.vectorStore.Flush(); err != nil {
			log.Error("failed to flush VectorDB", "error", err)
		}
	}()
}

// ContextMessage is a message for sharing context via P2P.
type ContextMessage struct {
	Type      string         `json:"type"`
	FilePath  string         `json:"file_path"`
	Content   string         `json:"content"`
	Embedding []float32      `json:"embedding,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	SourceID  string         `json:"source_id"`
}

// BroadcastContext broadcasts shared context to all peers.
func (a *App) BroadcastContext(filePath, content string, embedding []float32, metadata map[string]any) error {
	if a.node == nil {
		return fmt.Errorf("node not initialized")
	}

	msg := ContextMessage{
		Type:      "shared_context",
		FilePath:  filePath,
		Content:   content,
		Embedding: embedding,
		Metadata:  metadata,
		SourceID:  a.node.ID().String(),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	topicName := "/agent-collab/" + a.config.ProjectName + "/context"
	return a.node.Publish(a.ctx, topicName, data)
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

	// Initialize global cluster services (Interest Manager & Event Router)
	a.interestMgr = interest.NewManager()
	a.eventRouter = event.NewRouter(a.interestMgr, &event.RouterConfig{
		NodeID:      nodeID,
		NodeName:    nodeName,
		VectorStore: a.vectorStore,
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

// InterestManager returns the interest manager.
func (a *App) InterestManager() *interest.Manager {
	return a.interestMgr
}

// EventRouter returns the event router.
func (a *App) EventRouter() *event.Router {
	return a.eventRouter
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

// EventBridge returns the event bridge.
func (a *App) EventBridge() *libp2p.EventBridge {
	return a.eventBridge
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

	// WireGuard VPN info (optional)
	WireGuardEnabled  bool   `json:"wireguard_enabled,omitempty"`
	WireGuardIP       string `json:"wireguard_ip,omitempty"`
	WireGuardEndpoint string `json:"wireguard_endpoint,omitempty"`
}

// JoinResult는 참여 결과입니다.
type JoinResult struct {
	ProjectName    string `json:"project_name"`
	NodeID         string `json:"node_id"`
	BootstrapPeer  string `json:"bootstrap_peer"`
	ConnectedPeers int    `json:"connected_peers"`

	// WireGuard VPN info (optional)
	WireGuardEnabled bool   `json:"wireguard_enabled,omitempty"`
	WireGuardIP      string `json:"wireguard_ip,omitempty"`
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

	// WireGuard VPN status
	WireGuardEnabled   bool   `json:"wireguard_enabled,omitempty"`
	WireGuardIP        string `json:"wireguard_ip,omitempty"`
	WireGuardEndpoint  string `json:"wireguard_endpoint,omitempty"`
	WireGuardPeerCount int    `json:"wireguard_peer_count,omitempty"`
}

// Ensure libp2pcrypto is used
var _ libp2pcrypto.PrivKey = nil
