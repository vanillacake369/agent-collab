// Package platform provides platform-specific WireGuard implementations.
package platform

import (
	"net"
)

// Platform abstracts platform-specific WireGuard operations.
type Platform interface {
	// Name returns the platform name.
	Name() string

	// IsSupported checks if WireGuard is supported on this platform.
	IsSupported() bool

	// CreateInterface creates a new WireGuard interface.
	CreateInterface(name string) (Device, error)

	// DeleteInterface deletes an existing WireGuard interface.
	DeleteInterface(name string) error

	// RequiresRoot returns true if root/admin privileges are required.
	RequiresRoot() bool

	// GetExternalIP attempts to detect the external IP address.
	GetExternalIP() (string, error)
}

// Device represents a WireGuard network device.
type Device interface {
	// Name returns the device name.
	Name() string

	// Configure applies WireGuard configuration to the device.
	Configure(cfg *DeviceConfig) error

	// AddIP adds an IP address to the device.
	AddIP(ip string) error

	// RemoveIP removes an IP address from the device.
	RemoveIP(ip string) error

	// Up brings the interface up.
	Up() error

	// Down brings the interface down.
	Down() error

	// IsUp returns true if the interface is up.
	IsUp() bool

	// GetConfig returns the current device configuration.
	GetConfig() (*DeviceConfig, error)

	// Close closes the device.
	Close() error
}

// DeviceConfig holds the WireGuard device configuration.
type DeviceConfig struct {
	PrivateKey   []byte // #nosec G117 - WireGuard key, intentionally named
	ListenPort   int
	FirewallMark int
	Peers        []PeerConfig
}

// PeerConfig holds WireGuard peer configuration.
type PeerConfig struct {
	PublicKey                   []byte
	PresharedKey                []byte
	Endpoint                    *net.UDPAddr
	AllowedIPs                  []net.IPNet
	PersistentKeepaliveInterval int
	ReplaceAllowedIPs           bool
	RemoveMe                    bool
}

// PeerStats holds statistics for a WireGuard peer.
type PeerStats struct {
	PublicKey           []byte
	Endpoint            *net.UDPAddr
	LastHandshakeTime   int64
	ReceiveBytes        int64
	TransmitBytes       int64
	AllowedIPs          []net.IPNet
	ProtocolVersion     int
	PersistentKeepalive int
}

// GetPlatform returns the appropriate platform implementation.
func GetPlatform() Platform {
	return getPlatformImpl()
}

// Detect attempts to detect the best available WireGuard implementation.
func Detect() (string, bool) {
	p := GetPlatform()
	if p.IsSupported() {
		return p.Name(), true
	}
	return "", false
}
