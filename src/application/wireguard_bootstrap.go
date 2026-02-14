package application

import (
	"context"
	"fmt"
	"path/filepath"

	"agent-collab/src/infrastructure/crypto"
	"agent-collab/src/infrastructure/network/wireguard"
	"agent-collab/src/pkg/logging"
)

// WireGuardBootstrapper handles WireGuard VPN setup for both init and join scenarios.
type WireGuardBootstrapper struct {
	dataDir string
	config  *WireGuardConfig
	logger  *logging.Logger
}

// NewWireGuardBootstrapper creates a new bootstrapper.
func NewWireGuardBootstrapper(dataDir string, config *WireGuardConfig, logger *logging.Logger) *WireGuardBootstrapper {
	if config == nil {
		config = DefaultWireGuardConfig()
	}
	return &WireGuardBootstrapper{
		dataDir: dataDir,
		config:  config,
		logger:  logger,
	}
}

// BootstrapOptions configures the WireGuard bootstrap process.
type BootstrapOptions struct {
	// For init mode
	ListenPort int
	Subnet     string

	// For join mode
	CreatorPublicKey string
	CreatorEndpoint  string
	CreatorIP        string
}

// BootstrapResult contains the result of bootstrap operation.
type BootstrapResult struct {
	Manager *wireguard.WireGuardManager
	Info    *crypto.WireGuardInfo
}

// Bootstrap initializes WireGuard and returns the manager and info.
func (b *WireGuardBootstrapper) Bootstrap(ctx context.Context, opts *BootstrapOptions) (*BootstrapResult, error) {
	b.config.Enabled = true

	// Apply options
	if opts.ListenPort > 0 {
		b.config.ListenPort = opts.ListenPort
	}
	if opts.Subnet != "" {
		b.config.Subnet = opts.Subnet
	}

	// Create and initialize manager
	mgr := wireguard.NewManager(nil)
	mgrCfg := &wireguard.ManagerConfig{
		InterfaceName:       b.config.InterfaceName,
		ListenPort:          b.config.ListenPort,
		Subnet:              b.config.Subnet,
		MTU:                 b.config.MTU,
		PersistentKeepalive: b.config.PersistentKeepalive,
		AutoDetectEndpoint:  true,
	}

	if err := mgr.Initialize(ctx, mgrCfg); err != nil {
		return nil, fmt.Errorf("failed to initialize WireGuard manager: %w", err)
	}

	// Add creator peer if joining
	if opts.CreatorPublicKey != "" {
		creatorPeer := &wireguard.Peer{
			PublicKey:           opts.CreatorPublicKey,
			Endpoint:            opts.CreatorEndpoint,
			AllowedIPs:          []string{opts.CreatorIP},
			PersistentKeepalive: b.config.PersistentKeepalive,
		}
		if err := mgr.AddPeer(creatorPeer); err != nil {
			return nil, fmt.Errorf("failed to add creator peer: %w", err)
		}
	}

	// Start the WireGuard interface
	if err := mgr.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start WireGuard: %w", err)
	}

	// Save WireGuard config
	if err := b.saveConfig(mgr); err != nil {
		if b.logger != nil {
			b.logger.Warn("failed to save WireGuard config", "error", err)
		}
	}

	// Build result
	keyPair := mgr.GetKeyPair()
	result := &BootstrapResult{
		Manager: mgr,
		Info: &crypto.WireGuardInfo{
			CreatorPublicKey: keyPair.PublicKey,
			CreatorEndpoint:  mgr.GetEndpoint(),
			Subnet:           b.config.Subnet,
			CreatorIP:        mgr.GetLocalIP(),
		},
	}

	return result, nil
}

// saveConfig saves the WireGuard configuration to disk.
func (b *WireGuardBootstrapper) saveConfig(mgr *wireguard.WireGuardManager) error {
	wgConfigPath := filepath.Join(b.dataDir, "wireguard.json")
	return wireguard.SaveConfigFile(mgr.GetConfig().ToConfigFile(), wgConfigPath)
}
