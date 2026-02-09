//go:build darwin

package platform

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"golang.zx2c4.com/wireguard/wgctrl"
	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
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

	// macOS uses utun interfaces
	// wireguard-go creates the interface automatically
	// We need to find an available utun device

	// Start wireguard-go in userspace mode
	cmd := exec.Command("wireguard-go", name)
	cmd.Env = append(os.Environ(), "WG_TUN_NAME_FILE=/tmp/wg-"+name+".name")
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if already running
		if !strings.Contains(string(output), "already exists") {
			return nil, fmt.Errorf("failed to create interface: %s: %w", output, err)
		}
	}

	// Wait for interface to be ready
	time.Sleep(500 * time.Millisecond)

	// Open wgctrl client
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}

	return &darwinDevice{
		name:   name,
		client: client,
	}, nil
}

func (p *darwinPlatform) DeleteInterface(name string) error {
	// On macOS, we need to kill the wireguard-go process
	// This will automatically remove the utun interface

	// Find and kill wireguard-go process for this interface
	cmd := exec.Command("pkill", "-f", "wireguard-go "+name)
	_ = cmd.Run() // Ignore error if process doesn't exist

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
	name   string
	client *wgctrl.Client
}

func (d *darwinDevice) Name() string {
	return d.name
}

func (d *darwinDevice) Configure(cfg *DeviceConfig) error {
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

	// On macOS, use ifconfig
	cmd := exec.Command("ifconfig", d.name, "inet", ipAddr.String(), ipAddr.String(), "netmask", ipNetMask(ipNet))
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add IP: %s: %w", output, err)
	}

	// Add route for the subnet
	ones, _ := ipNet.Mask.Size()
	cmd = exec.Command("route", "-n", "add", "-net", fmt.Sprintf("%s/%d", ipNet.IP.String(), ones), "-interface", d.name)
	_ = cmd.Run() // Ignore error if route already exists

	return nil
}

func (d *darwinDevice) RemoveIP(ip string) error {
	ipAddr, _, err := net.ParseCIDR(ip)
	if err != nil {
		return fmt.Errorf("invalid IP: %w", err)
	}

	cmd := exec.Command("ifconfig", d.name, "inet", ipAddr.String(), "delete")
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "Can't assign") {
			return nil
		}
		return fmt.Errorf("failed to remove IP: %s: %w", output, err)
	}
	return nil
}

func (d *darwinDevice) Up() error {
	cmd := exec.Command("ifconfig", d.name, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface up: %s: %w", output, err)
	}
	return nil
}

func (d *darwinDevice) Down() error {
	cmd := exec.Command("ifconfig", d.name, "down")
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
