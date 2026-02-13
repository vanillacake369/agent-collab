package wireguard

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// ConfigFile represents a WireGuard configuration file.
type ConfigFile struct {
	// Interface configuration
	Interface InterfaceConfig `json:"interface"`
	// Peer configurations
	Peers []PeerConfig `json:"peers"`
}

// InterfaceConfig represents the [Interface] section of a WireGuard config.
type InterfaceConfig struct {
	PrivateKey string `json:"private_key"` // #nosec G117 - WireGuard key, intentionally named
	ListenPort int    `json:"listen_port,omitempty"`
	Address    string `json:"address"` // CIDR notation
	MTU        int    `json:"mtu,omitempty"`
	DNS        string `json:"dns,omitempty"`
}

// PeerConfig represents a [Peer] section of a WireGuard config.
type PeerConfig struct {
	PublicKey           string   `json:"public_key"`
	PresharedKey        string   `json:"preshared_key,omitempty"`
	Endpoint            string   `json:"endpoint,omitempty"`
	AllowedIPs          []string `json:"allowed_ips"`
	PersistentKeepalive int      `json:"persistent_keepalive,omitempty"`
}

// LoadConfigFile loads a WireGuard configuration from a JSON file.
func LoadConfigFile(path string) (*ConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg ConfigFile
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return &cfg, nil
}

// SaveConfigFile saves a WireGuard configuration to a JSON file.
func SaveConfigFile(cfg *ConfigFile, path string) error {
	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write with restrictive permissions (config contains private key)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// ToConfig converts ConfigFile to the internal Config type.
func (cf *ConfigFile) ToConfig() (*Config, error) {
	publicKey, err := DerivePublicKey(cf.Interface.PrivateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to derive public key: %w", err)
	}

	// Extract subnet from address
	_, subnet, err := net.ParseCIDR(cf.Interface.Address)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}

	cfg := &Config{
		PrivateKey: cf.Interface.PrivateKey,
		PublicKey:  publicKey,
		ListenPort: cf.Interface.ListenPort,
		LocalIP:    cf.Interface.Address,
		Subnet:     subnet.String(),
		MTU:        cf.Interface.MTU,
		Peers:      make([]*Peer, len(cf.Peers)),
	}

	if cfg.MTU == 0 {
		cfg.MTU = 1420 // Default MTU
	}

	for i, p := range cf.Peers {
		cfg.Peers[i] = &Peer{
			PublicKey:           p.PublicKey,
			AllowedIPs:          p.AllowedIPs,
			Endpoint:            p.Endpoint,
			PersistentKeepalive: p.PersistentKeepalive,
		}
	}

	return cfg, cfg.Validate()
}

// ToConfigFile converts Config to ConfigFile format.
func (c *Config) ToConfigFile() *ConfigFile {
	cf := &ConfigFile{
		Interface: InterfaceConfig{
			PrivateKey: c.PrivateKey,
			ListenPort: c.ListenPort,
			Address:    c.LocalIP,
			MTU:        c.MTU,
		},
		Peers: make([]PeerConfig, len(c.Peers)),
	}

	for i, p := range c.Peers {
		cf.Peers[i] = PeerConfig{
			PublicKey:           p.PublicKey,
			AllowedIPs:          p.AllowedIPs,
			Endpoint:            p.Endpoint,
			PersistentKeepalive: p.PersistentKeepalive,
		}
	}

	return cf
}

// ValidateEndpoint validates a WireGuard endpoint string (host:port).
func ValidateEndpoint(endpoint string) error {
	if endpoint == "" {
		return nil // Empty endpoint is valid (peer without fixed endpoint)
	}

	host, port, err := net.SplitHostPort(endpoint)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEndpoint, err)
	}

	// Validate host is IP or hostname
	if net.ParseIP(host) == nil {
		// Not an IP, check if it's a valid hostname
		if len(host) == 0 || len(host) > 253 {
			return ErrInvalidEndpoint
		}
	}

	// Validate port
	if _, err := net.LookupPort("udp", port); err != nil {
		return fmt.Errorf("%w: invalid port", ErrInvalidEndpoint)
	}

	return nil
}

// ParseAllowedIPs parses and validates a list of allowed IPs.
func ParseAllowedIPs(allowedIPs []string) ([]net.IPNet, error) {
	result := make([]net.IPNet, 0, len(allowedIPs))

	for _, cidr := range allowedIPs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			return nil, fmt.Errorf("invalid allowed IP %q: %w", cidr, err)
		}
		result = append(result, *ipnet)
	}

	return result, nil
}
