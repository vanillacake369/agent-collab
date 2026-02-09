package platform

import (
	"fmt"
	"net"
	"sync"
)

// MockPlatform provides a mock WireGuard platform for testing.
type MockPlatform struct {
	devices map[string]*MockDevice
	mu      sync.Mutex
}

// NewMockPlatform creates a new mock platform.
func NewMockPlatform() *MockPlatform {
	return &MockPlatform{
		devices: make(map[string]*MockDevice),
	}
}

func (p *MockPlatform) Name() string {
	return "mock"
}

func (p *MockPlatform) IsSupported() bool {
	return true
}

func (p *MockPlatform) RequiresRoot() bool {
	return false
}

func (p *MockPlatform) CreateInterface(name string) (Device, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.devices[name]; exists {
		return nil, fmt.Errorf("interface %s already exists", name)
	}

	device := &MockDevice{
		name:   name,
		ips:    make([]string, 0),
		peers:  make([]PeerConfig, 0),
		isUp:   false,
		events: make([]MockEvent, 0),
	}
	p.devices[name] = device

	return device, nil
}

func (p *MockPlatform) DeleteInterface(name string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if _, exists := p.devices[name]; !exists {
		return nil
	}

	delete(p.devices, name)
	return nil
}

func (p *MockPlatform) GetExternalIP() (string, error) {
	return "192.168.1.100", nil
}

// GetDevice returns a mock device by name (for testing).
func (p *MockPlatform) GetDevice(name string) (*MockDevice, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	d, ok := p.devices[name]
	return d, ok
}

// MockDevice is a mock WireGuard device for testing.
type MockDevice struct {
	mu         sync.Mutex
	name       string
	privateKey []byte
	listenPort int
	ips        []string
	peers      []PeerConfig
	isUp       bool
	events     []MockEvent
}

// MockEvent represents an event on the mock device.
type MockEvent struct {
	Type string
	Data interface{}
}

func (d *MockDevice) Name() string {
	return d.name
}

func (d *MockDevice) Configure(cfg *DeviceConfig) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.privateKey = cfg.PrivateKey
	d.listenPort = cfg.ListenPort

	// Process peer configurations
	for _, peer := range cfg.Peers {
		if peer.RemoveMe {
			d.removePeer(peer.PublicKey)
			continue
		}
		d.addOrUpdatePeer(peer)
	}

	d.events = append(d.events, MockEvent{Type: "configure", Data: cfg})
	return nil
}

func (d *MockDevice) addOrUpdatePeer(peer PeerConfig) {
	for i, p := range d.peers {
		if string(p.PublicKey) == string(peer.PublicKey) {
			d.peers[i] = peer
			return
		}
	}
	d.peers = append(d.peers, peer)
}

func (d *MockDevice) removePeer(publicKey []byte) {
	for i, p := range d.peers {
		if string(p.PublicKey) == string(publicKey) {
			d.peers = append(d.peers[:i], d.peers[i+1:]...)
			return
		}
	}
}

func (d *MockDevice) AddIP(ip string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, existing := range d.ips {
		if existing == ip {
			return nil // Already exists
		}
	}

	d.ips = append(d.ips, ip)
	d.events = append(d.events, MockEvent{Type: "add_ip", Data: ip})
	return nil
}

func (d *MockDevice) RemoveIP(ip string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, existing := range d.ips {
		if existing == ip {
			d.ips = append(d.ips[:i], d.ips[i+1:]...)
			d.events = append(d.events, MockEvent{Type: "remove_ip", Data: ip})
			return nil
		}
	}
	return nil
}

func (d *MockDevice) Up() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.isUp = true
	d.events = append(d.events, MockEvent{Type: "up", Data: nil})
	return nil
}

func (d *MockDevice) Down() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.isUp = false
	d.events = append(d.events, MockEvent{Type: "down", Data: nil})
	return nil
}

func (d *MockDevice) IsUp() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.isUp
}

func (d *MockDevice) GetConfig() (*DeviceConfig, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	cfg := &DeviceConfig{
		PrivateKey: d.privateKey,
		ListenPort: d.listenPort,
		Peers:      make([]PeerConfig, len(d.peers)),
	}
	copy(cfg.Peers, d.peers)
	return cfg, nil
}

func (d *MockDevice) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.events = append(d.events, MockEvent{Type: "close", Data: nil})
	return nil
}

// GetIPs returns the configured IPs (for testing).
func (d *MockDevice) GetIPs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	result := make([]string, len(d.ips))
	copy(result, d.ips)
	return result
}

// GetPeers returns the configured peers (for testing).
func (d *MockDevice) GetPeers() []PeerConfig {
	d.mu.Lock()
	defer d.mu.Unlock()
	result := make([]PeerConfig, len(d.peers))
	copy(result, d.peers)
	return result
}

// GetEvents returns recorded events (for testing).
func (d *MockDevice) GetEvents() []MockEvent {
	d.mu.Lock()
	defer d.mu.Unlock()
	result := make([]MockEvent, len(d.events))
	copy(result, d.events)
	return result
}

// ClearEvents clears recorded events (for testing).
func (d *MockDevice) ClearEvents() {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.events = make([]MockEvent, 0)
}

// Ensure MockDevice implements Device
var _ Device = (*MockDevice)(nil)

// Ensure MockPlatform implements Platform
var _ Platform = (*MockPlatform)(nil)

// Ensure net package is used
var _ = net.IPv4(0, 0, 0, 0)
