package wireguard

import "context"

// Interface defines the WireGuard interface operations.
type Interface interface {
	// Configure applies the WireGuard configuration.
	Configure(cfg *Config) error

	// Up brings the interface up.
	Up() error

	// Down brings the interface down.
	Down() error

	// AddPeer adds a new peer to the interface.
	AddPeer(peer *Peer) error

	// RemovePeer removes a peer from the interface.
	RemovePeer(publicKey string) error

	// UpdatePeer updates an existing peer's configuration.
	UpdatePeer(peer *Peer) error

	// ListPeers returns all configured peers.
	ListPeers() ([]*Peer, error)

	// GetStatus returns the interface status.
	GetStatus() (*InterfaceStatus, error)

	// GetLocalIP returns the local VPN IP address.
	GetLocalIP() string

	// Close shuts down the interface.
	Close() error
}

// Manager defines the WireGuard manager interface.
type Manager interface {
	// Initialize sets up the WireGuard manager.
	Initialize(ctx context.Context, cfg *ManagerConfig) error

	// Start starts the WireGuard interface.
	Start(ctx context.Context) error

	// Stop stops the WireGuard interface.
	Stop() error

	// AddPeer adds a new peer.
	AddPeer(peer *Peer) error

	// RemovePeer removes a peer by public key.
	RemovePeer(publicKey string) error

	// AllocateIP allocates an IP for a peer.
	AllocateIP(peerID string) (string, error)

	// ReleaseIP releases an IP for a peer.
	ReleaseIP(peerID string) error

	// GetConfig returns the current configuration.
	GetConfig() *Config

	// GetStatus returns the current status.
	GetStatus() (*InterfaceStatus, error)

	// GetKeyPair returns the WireGuard key pair.
	GetKeyPair() *KeyPair

	// GetLocalIP returns the local VPN IP.
	GetLocalIP() string

	// GetEndpoint returns the external endpoint (IP:port).
	GetEndpoint() string

	// IsRunning returns true if the manager is running.
	IsRunning() bool

	// Close shuts down the manager.
	Close() error
}
