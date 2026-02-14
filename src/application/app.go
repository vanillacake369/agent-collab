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
		tok, err := crypto.NewWireGuardToken(addrStrs, opts.ProjectName, nodeIDStr, wgInfo)
		if err != nil {
			return nil, fmt.Errorf("failed to create wireguard token: %w", err)
		}
		tokenStr, err = tok.Encode()
		if err != nil {
			return nil, fmt.Errorf("failed to encode wireguard token: %w", err)
		}
	} else {
		// Create simple token
		tok, err := crypto.NewInviteToken(addrStrs, opts.ProjectName, nodeIDStr)
		if err != nil {
			return nil, fmt.Errorf("failed to create invite token: %w", err)
		}
		tokenStr, err = tok.Encode()
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
	bootstrapper := NewWireGuardBootstrapper(a.config.DataDir, a.config.WireGuard, a.logger)

	result, err := bootstrapper.Bootstrap(ctx, &BootstrapOptions{
		ListenPort: opts.WireGuardPort,
		Subnet:     opts.Subnet,
	})
	if err != nil {
		return nil, err
	}

	a.wgManager = result.Manager
	a.config.WireGuard = bootstrapper.config

	return result.Info, nil
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
	tok, hasWireGuard, err := crypto.DecodeAnyToken(tokenStr)
	if err != nil {
		return nil, fmt.Errorf("invalid invite token: %w", err)
	}

	// Check token expiration
	if tok.IsExpired() {
		return nil, fmt.Errorf("invite token has expired")
	}

	a.config.ProjectName = tok.ProjectName

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
	if hasWireGuard && tok.WireGuard != nil {
		if err := a.joinWithWireGuard(ctx, tok.WireGuard); err != nil {
			// Log warning but continue with libp2p-only mode
			a.logger.Warn("WireGuard setup failed, using libp2p only", "error", err)
		}
	}

	// 4. Bootstrap peer 주소 파싱
	var bootstrapPeers []peer.AddrInfo
	for _, addrStr := range tok.Addresses {
		ma, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			continue
		}
		peerInfo, err := peer.AddrInfoFromP2pAddr(ma)
		if err != nil {
			// 주소에 peer ID가 없으면 토큰의 CreatorID 사용
			creatorID, err := peer.Decode(tok.CreatorID)
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
	a.lockService = lock.NewLockService(ctx, nodeIDStr, tok.ProjectName+"-agent")
	a.syncManager = ctxsync.NewSyncManager(nodeIDStr, tok.ProjectName+"-agent")

	// 7. Initialize Phase 3 components
	if err := a.initPhase3Components(nodeIDStr, tok.ProjectName+"-agent"); err != nil {
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
	a.config.Bootstrap = tok.Addresses
	a.config.BootstrapPeer = tok.CreatorID

	// Save config for daemon to load later
	if err := a.saveConfig(); err != nil {
		a.logger.Warn("failed to save config", "error", err)
	}

	result := &JoinResult{
		ProjectName:    tok.ProjectName,
		NodeID:         nodeIDStr,
		BootstrapPeer:  tok.CreatorID,
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
	bootstrapper := NewWireGuardBootstrapper(a.config.DataDir, a.config.WireGuard, a.logger)

	result, err := bootstrapper.Bootstrap(ctx, &BootstrapOptions{
		Subnet:           wgInfo.Subnet,
		CreatorPublicKey: wgInfo.CreatorPublicKey,
		CreatorEndpoint:  wgInfo.CreatorEndpoint,
		CreatorIP:        wgInfo.CreatorIP,
	})
	if err != nil {
		return err
	}

	a.wgManager = result.Manager
	a.config.WireGuard = bootstrapper.config

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

// Ensure libp2pcrypto is used
var _ libp2pcrypto.PrivKey = nil
