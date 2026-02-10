//go:build darwin

package platform

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

const (
	// wireguard-go uses /var/run/wireguard/ for Unix sockets on macOS
	wgSocketDir = "/var/run/wireguard"
	// Maximum time to wait for wireguard-go to be ready
	maxWaitTime = 5 * time.Second
	// Poll interval for checking socket readiness
	pollInterval = 100 * time.Millisecond
)

func getPlatformImpl() Platform {
	return &darwinPlatform{}
}

type darwinPlatform struct{}

func (p *darwinPlatform) Name() string {
	return "darwin"
}

func (p *darwinPlatform) IsSupported() bool {
	// Check if wireguard-go is installed
	if _, err := exec.LookPath("wireguard-go"); err == nil {
		return true
	}
	// Check if wg-quick is available (from wireguard-tools)
	if _, err := exec.LookPath("wg-quick"); err == nil {
		return true
	}
	return false
}

func (p *darwinPlatform) RequiresRoot() bool {
	return os.Geteuid() != 0
}

func (p *darwinPlatform) CreateInterface(name string) (Device, error) {
	if p.RequiresRoot() {
		return nil, fmt.Errorf("wireguard: root privileges required")
	}

	// Step 1: Clean up any existing wireguard-go processes and sockets
	p.cleanupExisting(name)

	// Step 2: Ensure socket directory exists
	if err := os.MkdirAll(wgSocketDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %w", err)
	}

	// Step 3: macOS requires utun interface names
	// wireguard-go will auto-allocate utun device when given "utun"
	utunName := "utun"

	// Create a temp file to capture the actual interface name
	nameFile := fmt.Sprintf("/tmp/wg-%s.name", name)

	// Step 4: Start wireguard-go in userspace mode
	cmd := exec.Command("wireguard-go", utunName)
	cmd.Env = append(os.Environ(), "WG_TUN_NAME_FILE="+nameFile)

	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if already running
		if !strings.Contains(string(output), "already exists") {
			return nil, fmt.Errorf("failed to create interface: %s: %w", output, err)
		}
	}

	// Step 5: Read the actual interface name from the name file (with retry)
	actualName, err := p.waitForNameFile(nameFile, maxWaitTime)
	if err != nil {
		p.cleanupExisting(name)
		return nil, fmt.Errorf("failed to get interface name: %w", err)
	}

	// Clean up name file
	os.Remove(nameFile)

	// Step 6: Wait for the Unix socket to be ready
	socketPath := filepath.Join(wgSocketDir, actualName+".sock")
	if err := p.waitForSocket(socketPath, maxWaitTime); err != nil {
		p.cleanupExisting(name)
		return nil, fmt.Errorf("wireguard-go socket not ready: %w", err)
	}

	// Step 7: Open wgctrl client with retry
	client, err := p.createClientWithRetry(3)
	if err != nil {
		p.cleanupExisting(name)
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}

	// Step 8: Verify device is accessible
	if _, err := client.Device(actualName); err != nil {
		client.Close()
		p.cleanupExisting(name)
		return nil, fmt.Errorf("device not accessible: %w", err)
	}

	return &darwinDevice{
		name:       actualName,
		configName: name,
		client:     client,
	}, nil
}

// cleanupExisting kills any existing wireguard-go processes and removes sockets
func (p *darwinPlatform) cleanupExisting(name string) {
	// Kill wireguard-go processes
	exec.Command("pkill", "-9", "-f", "wireguard-go").Run()

	// Wait a bit for processes to terminate
	time.Sleep(200 * time.Millisecond)

	// Remove any existing sockets
	if files, err := filepath.Glob(filepath.Join(wgSocketDir, "*.sock")); err == nil {
		for _, f := range files {
			os.Remove(f)
		}
	}

	// Remove name file
	os.Remove(fmt.Sprintf("/tmp/wg-%s.name", name))
}

// waitForNameFile waits for the name file to be written and returns its contents
func (p *darwinPlatform) waitForNameFile(path string, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if data, err := os.ReadFile(path); err == nil {
			name := strings.TrimSpace(string(data))
			if name != "" {
				return name, nil
			}
		}
		time.Sleep(pollInterval)
	}

	return "", fmt.Errorf("timeout waiting for name file")
}

// waitForSocket waits for the Unix socket to be created and accessible
func (p *darwinPlatform) waitForSocket(path string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if info, err := os.Stat(path); err == nil {
			// Check if it's a socket
			if info.Mode()&os.ModeSocket != 0 {
				return nil
			}
		}
		time.Sleep(pollInterval)
	}

	return fmt.Errorf("timeout waiting for socket: %s", path)
}

// createClientWithRetry creates a wgctrl client with retry logic
func (p *darwinPlatform) createClientWithRetry(maxAttempts int) (*wgctrl.Client, error) {
	var lastErr error

	for i := 0; i < maxAttempts; i++ {
		client, err := wgctrl.New()
		if err == nil {
			return client, nil
		}
		lastErr = err

		// Wait before retry
		time.Sleep(500 * time.Millisecond)
	}

	return nil, lastErr
}

func (p *darwinPlatform) DeleteInterface(name string) error {
	// On macOS, we need to kill the wireguard-go process
	// This will automatically remove the utun interface

	// Find and kill wireguard-go processes
	exec.Command("pkill", "-9", "-f", "wireguard-go").Run()

	// Wait for cleanup
	time.Sleep(200 * time.Millisecond)

	// Clean up socket
	if files, err := filepath.Glob(filepath.Join(wgSocketDir, "*.sock")); err == nil {
		for _, f := range files {
			os.Remove(f)
		}
	}

	// Clean up name file
	os.Remove(fmt.Sprintf("/tmp/wg-%s.name", name))

	return nil
}

func (p *darwinPlatform) GetExternalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("failed to detect external IP: %w", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

type darwinDevice struct {
	name       string // Actual utun interface name (e.g., utun5)
	configName string // Original requested name for reference
	client     *wgctrl.Client
}

func (d *darwinDevice) Name() string {
	return d.name
}

func (d *darwinDevice) Configure(cfg *DeviceConfig) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	privateKey, err := wgtypes.NewKey(cfg.PrivateKey)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	wgCfg := wgtypes.Config{
		PrivateKey:   &privateKey,
		ListenPort:   &cfg.ListenPort,
		ReplacePeers: false,
		Peers:        make([]wgtypes.PeerConfig, 0, len(cfg.Peers)),
	}

	for _, peer := range cfg.Peers {
		publicKey, err := wgtypes.NewKey(peer.PublicKey)
		if err != nil {
			return fmt.Errorf("invalid peer public key: %w", err)
		}

		peerCfg := wgtypes.PeerConfig{
			PublicKey:         publicKey,
			Endpoint:          peer.Endpoint,
			ReplaceAllowedIPs: peer.ReplaceAllowedIPs,
			Remove:            peer.RemoveMe,
		}

		if len(peer.PresharedKey) > 0 {
			psk, err := wgtypes.NewKey(peer.PresharedKey)
			if err != nil {
				return fmt.Errorf("invalid preshared key: %w", err)
			}
			peerCfg.PresharedKey = &psk
		}

		if peer.PersistentKeepaliveInterval > 0 {
			keepalive := time.Duration(peer.PersistentKeepaliveInterval) * time.Second
			peerCfg.PersistentKeepaliveInterval = &keepalive
		}

		peerCfg.AllowedIPs = make([]net.IPNet, len(peer.AllowedIPs))
		copy(peerCfg.AllowedIPs, peer.AllowedIPs)

		wgCfg.Peers = append(wgCfg.Peers, peerCfg)
	}

	return d.client.ConfigureDevice(d.name, wgCfg)
}

func (d *darwinDevice) AddIP(ip string) error {
	// Parse CIDR
	ipAddr, ipNet, err := net.ParseCIDR(ip)
	if err != nil {
		return fmt.Errorf("invalid IP: %w", err)
	}

	// On macOS, utun is a point-to-point interface
	// Format: /sbin/ifconfig <interface> inet <local_addr> <dest_addr> netmask <mask>
	// For WireGuard, we use the same address for both local and destination
	cmd := exec.Command("/sbin/ifconfig", d.name, "inet", ipAddr.String(), ipAddr.String(), "netmask", ipNetMask(ipNet))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add IP: %s: %w", output, err)
	}

	// Add route for the subnet
	ones, _ := ipNet.Mask.Size()
	cmd = exec.Command("/sbin/route", "-n", "add", "-net", fmt.Sprintf("%s/%d", ipNet.IP.String(), ones), "-interface", d.name)
	_ = cmd.Run() // Ignore error if route already exists

	return nil
}

func (d *darwinDevice) RemoveIP(ip string) error {
	ipAddr, _, err := net.ParseCIDR(ip)
	if err != nil {
		return fmt.Errorf("invalid IP: %w", err)
	}

	cmd := exec.Command("/sbin/ifconfig", d.name, "inet", ipAddr.String(), "delete")
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "Can't assign") {
			return nil
		}
		return fmt.Errorf("failed to remove IP: %s: %w", output, err)
	}
	return nil
}

func (d *darwinDevice) Up() error {
	cmd := exec.Command("/sbin/ifconfig", d.name, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface up: %s: %w", output, err)
	}
	return nil
}

func (d *darwinDevice) Down() error {
	cmd := exec.Command("/sbin/ifconfig", d.name, "down")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface down: %s: %w", output, err)
	}
	return nil
}

func (d *darwinDevice) IsUp() bool {
	iface, err := net.InterfaceByName(d.name)
	if err != nil {
		return false
	}
	return iface.Flags&net.FlagUp != 0
}

func (d *darwinDevice) GetConfig() (*DeviceConfig, error) {
	device, err := d.client.Device(d.name)
	if err != nil {
		return nil, fmt.Errorf("failed to get device config: %w", err)
	}

	cfg := &DeviceConfig{
		PrivateKey: device.PrivateKey[:],
		ListenPort: device.ListenPort,
		Peers:      make([]PeerConfig, 0, len(device.Peers)),
	}

	for _, peer := range device.Peers {
		peerCfg := PeerConfig{
			PublicKey:  peer.PublicKey[:],
			Endpoint:   peer.Endpoint,
			AllowedIPs: peer.AllowedIPs,
		}
		if peer.PresharedKey != (wgtypes.Key{}) {
			peerCfg.PresharedKey = peer.PresharedKey[:]
		}
		if peer.PersistentKeepaliveInterval > 0 {
			peerCfg.PersistentKeepaliveInterval = int(peer.PersistentKeepaliveInterval.Seconds())
		}
		cfg.Peers = append(cfg.Peers, peerCfg)
	}

	return cfg, nil
}

func (d *darwinDevice) Close() error {
	return d.client.Close()
}

// ipNetMask converts an IPNet mask to dotted decimal format
func ipNetMask(ipNet *net.IPNet) string {
	mask := ipNet.Mask
	if len(mask) == 4 {
		return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
	}
	// For IPv6, return hex representation
	return mask.String()
}
