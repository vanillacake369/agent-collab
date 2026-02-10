//go:build linux

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
	return &linuxPlatform{}
}

type linuxPlatform struct{}

func (p *linuxPlatform) Name() string {
	return "linux"
}

func (p *linuxPlatform) IsSupported() bool {
	// Check if wireguard kernel module is available or wireguard-go
	if _, err := exec.LookPath("wg"); err == nil {
		return true
	}
	// Check if we can load the module
	if _, err := os.Stat("/sys/module/wireguard"); err == nil {
		return true
	}
	return false
}

func (p *linuxPlatform) RequiresRoot() bool {
	return os.Geteuid() != 0
}

func (p *linuxPlatform) CreateInterface(name string) (Device, error) {
	if p.RequiresRoot() {
		return nil, fmt.Errorf("wireguard: root privileges required")
	}

	// Create WireGuard interface using ip command
	cmd := exec.Command("ip", "link", "add", "dev", name, "type", "wireguard")
	if output, err := cmd.CombinedOutput(); err != nil {
		// Check if interface already exists
		if strings.Contains(string(output), "File exists") {
			// Interface exists, try to use it
		} else {
			return nil, fmt.Errorf("failed to create interface: %s: %w", output, err)
		}
	}

	// Open wgctrl client
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}

	return &linuxDevice{
		name:   name,
		client: client,
	}, nil
}

func (p *linuxPlatform) DeleteInterface(name string) error {
	cmd := exec.Command("ip", "link", "del", "dev", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "Cannot find device") {
			return nil // Already deleted
		}
		return fmt.Errorf("failed to delete interface: %s: %w", output, err)
	}
	return nil
}

func (p *linuxPlatform) GetExternalIP() (string, error) {
	// Try to detect external IP by checking default route interface
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("failed to detect external IP: %w", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

type linuxDevice struct {
	name   string
	client *wgctrl.Client
}

func (d *linuxDevice) Name() string {
	return d.name
}

func (d *linuxDevice) Configure(cfg *DeviceConfig) error {
	privateKey, err := wgtypes.NewKey(cfg.PrivateKey)
	if err != nil {
		return fmt.Errorf("invalid private key: %w", err)
	}

	wgCfg := wgtypes.Config{
		PrivateKey:   &privateKey,
		ListenPort:   &cfg.ListenPort,
		FirewallMark: &cfg.FirewallMark,
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

		// Convert AllowedIPs
		peerCfg.AllowedIPs = make([]net.IPNet, len(peer.AllowedIPs))
		copy(peerCfg.AllowedIPs, peer.AllowedIPs)

		wgCfg.Peers = append(wgCfg.Peers, peerCfg)
	}

	return d.client.ConfigureDevice(d.name, wgCfg)
}

func (d *linuxDevice) AddIP(ip string) error {
	cmd := exec.Command("ip", "addr", "add", ip, "dev", d.name)
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "File exists") {
			return nil // Already has this IP
		}
		return fmt.Errorf("failed to add IP: %s: %w", output, err)
	}
	return nil
}

func (d *linuxDevice) RemoveIP(ip string) error {
	cmd := exec.Command("ip", "addr", "del", ip, "dev", d.name)
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "Cannot assign") {
			return nil // IP doesn't exist
		}
		return fmt.Errorf("failed to remove IP: %s: %w", output, err)
	}
	return nil
}

func (d *linuxDevice) Up() error {
	cmd := exec.Command("ip", "link", "set", d.name, "up")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface up: %s: %w", output, err)
	}
	return nil
}

func (d *linuxDevice) Down() error {
	cmd := exec.Command("ip", "link", "set", d.name, "down")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface down: %s: %w", output, err)
	}
	return nil
}

func (d *linuxDevice) IsUp() bool {
	iface, err := net.InterfaceByName(d.name)
	if err != nil {
		return false
	}
	return iface.Flags&net.FlagUp != 0
}

func (d *linuxDevice) GetConfig() (*DeviceConfig, error) {
	device, err := d.client.Device(d.name)
	if err != nil {
		return nil, fmt.Errorf("failed to get device config: %w", err)
	}

	cfg := &DeviceConfig{
		PrivateKey:   device.PrivateKey[:],
		ListenPort:   device.ListenPort,
		FirewallMark: device.FirewallMark,
		Peers:        make([]PeerConfig, 0, len(device.Peers)),
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

func (d *linuxDevice) Close() error {
	return d.client.Close()
}
