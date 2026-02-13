// Package wireguard provides WireGuard VPN integration for agent-collab.
package wireguard

import (
	"net"
	"time"
)

// Version is the wireguard package version.
const Version = "1.0.0"

// Config holds WireGuard interface configuration.
type Config struct {
	PrivateKey string  `json:"private_key"` // #nosec G117 - WireGuard key, intentionally named
	PublicKey  string  `json:"public_key"`
	ListenPort int     `json:"listen_port"`
	LocalIP    string  `json:"local_ip"` // e.g., "10.100.0.1/24"
	Subnet     string  `json:"subnet"`   // e.g., "10.100.0.0/24"
	MTU        int     `json:"mtu"`
	Peers      []*Peer `json:"peers"`
}

// Peer represents a WireGuard peer.
type Peer struct {
	PublicKey           string   `json:"public_key"`
	AllowedIPs          []string `json:"allowed_ips"`
	Endpoint            string   `json:"endpoint,omitempty"`
	PersistentKeepalive int      `json:"persistent_keepalive"`
}

// KeyPair holds a WireGuard Curve25519 key pair.
type KeyPair struct {
	PrivateKey string `json:"private_key"` // #nosec G117 - WireGuard key, intentionally named
	PublicKey  string `json:"public_key"`
}

// InterfaceStatus represents the current state of a WireGuard interface.
type InterfaceStatus struct {
	Name       string       `json:"name"`
	PublicKey  string       `json:"public_key"`
	ListenPort int          `json:"listen_port"`
	LocalIP    string       `json:"local_ip"`
	Up         bool         `json:"up"`
	Peers      []PeerStatus `json:"peers"`
}

// PeerStatus represents the current state of a WireGuard peer.
type PeerStatus struct {
	PublicKey           string    `json:"public_key"`
	Endpoint            string    `json:"endpoint,omitempty"`
	AllowedIPs          []string  `json:"allowed_ips"`
	LastHandshake       time.Time `json:"last_handshake"`
	TransferRx          int64     `json:"transfer_rx"`
	TransferTx          int64     `json:"transfer_tx"`
	PersistentKeepalive int       `json:"persistent_keepalive"`
}

// DefaultConfig returns a default WireGuard configuration.
func DefaultConfig() *Config {
	return &Config{
		ListenPort: 51820,
		Subnet:     "10.100.0.0/24",
		MTU:        1420,
		Peers:      make([]*Peer, 0),
	}
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.PrivateKey == "" {
		return ErrInvalidPrivateKey
	}
	if c.PublicKey == "" {
		return ErrInvalidPublicKey
	}
	if c.ListenPort < 0 || c.ListenPort > 65535 {
		return ErrInvalidPort
	}
	if c.LocalIP == "" {
		return ErrInvalidLocalIP
	}

	// Validate LocalIP is valid CIDR
	_, _, err := net.ParseCIDR(c.LocalIP)
	if err != nil {
		return ErrInvalidLocalIP
	}

	// Validate Subnet is valid CIDR
	if c.Subnet != "" {
		_, _, err := net.ParseCIDR(c.Subnet)
		if err != nil {
			return ErrInvalidSubnet
		}
	}

	if c.MTU < 576 || c.MTU > 65535 {
		return ErrInvalidMTU
	}

	return nil
}

// Clone returns a deep copy of the configuration.
func (c *Config) Clone() *Config {
	clone := &Config{
		PrivateKey: c.PrivateKey,
		PublicKey:  c.PublicKey,
		ListenPort: c.ListenPort,
		LocalIP:    c.LocalIP,
		Subnet:     c.Subnet,
		MTU:        c.MTU,
		Peers:      make([]*Peer, len(c.Peers)),
	}
	for i, p := range c.Peers {
		clone.Peers[i] = p.Clone()
	}
	return clone
}

// Clone returns a deep copy of the peer.
func (p *Peer) Clone() *Peer {
	clone := &Peer{
		PublicKey:           p.PublicKey,
		Endpoint:            p.Endpoint,
		PersistentKeepalive: p.PersistentKeepalive,
		AllowedIPs:          make([]string, len(p.AllowedIPs)),
	}
	copy(clone.AllowedIPs, p.AllowedIPs)
	return clone
}

// ManagerConfig holds configuration for the WireGuard manager.
type ManagerConfig struct {
	InterfaceName       string `json:"interface_name"`
	ListenPort          int    `json:"listen_port"`
	Subnet              string `json:"subnet"`
	MTU                 int    `json:"mtu"`
	PersistentKeepalive int    `json:"persistent_keepalive"`
	AutoDetectEndpoint  bool   `json:"auto_detect_endpoint"`
}

// DefaultManagerConfig returns a default manager configuration.
func DefaultManagerConfig() *ManagerConfig {
	return &ManagerConfig{
		InterfaceName:       "wg-agent",
		ListenPort:          51820,
		Subnet:              "10.100.0.0/24",
		MTU:                 1420,
		PersistentKeepalive: 25,
		AutoDetectEndpoint:  true,
	}
}
