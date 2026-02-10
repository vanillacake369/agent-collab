//go:build windows

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
	return &windowsPlatform{}
}

type windowsPlatform struct{}

func (p *windowsPlatform) Name() string {
	return "windows"
}

func (p *windowsPlatform) IsSupported() bool {
	// Check if WireGuard is installed
	if _, err := exec.LookPath("wireguard.exe"); err == nil {
		return true
	}
	// Check common installation path
	if _, err := os.Stat("C:\\Program Files\\WireGuard\\wireguard.exe"); err == nil {
		return true
	}
	return false
}

func (p *windowsPlatform) RequiresRoot() bool {
	// On Windows, we need administrator privileges
	// Check if we're running as admin by trying to open a privileged handle
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err != nil
}

func (p *windowsPlatform) CreateInterface(name string) (Device, error) {
	if p.RequiresRoot() {
		return nil, fmt.Errorf("wireguard: administrator privileges required")
	}

	// Windows uses wintun driver
	// WireGuard for Windows manages this automatically
	// We'll use wireguard-go for userspace implementation

	cmd := exec.Command("wireguard-go.exe", name)
	if output, err := cmd.CombinedOutput(); err != nil {
		if !strings.Contains(string(output), "already exists") {
			return nil, fmt.Errorf("failed to create interface: %s: %w", output, err)
		}
	}

	// Wait for interface to be ready
	time.Sleep(1 * time.Second)

	// Open wgctrl client
	client, err := wgctrl.New()
	if err != nil {
		return nil, fmt.Errorf("failed to create wgctrl client: %w", err)
	}

	return &windowsDevice{
		name:   name,
		client: client,
	}, nil
}

func (p *windowsPlatform) DeleteInterface(name string) error {
	// On Windows, stop the WireGuard tunnel service
	cmd := exec.Command("net", "stop", "WireGuardTunnel$"+name)
	_ = cmd.Run() // Ignore error if service doesn't exist

	// Kill wireguard-go process if running
	cmd = exec.Command("taskkill", "/F", "/IM", "wireguard-go.exe")
	_ = cmd.Run()

	return nil
}

func (p *windowsPlatform) GetExternalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", fmt.Errorf("failed to detect external IP: %w", err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), nil
}

type windowsDevice struct {
	name   string
	client *wgctrl.Client
}

func (d *windowsDevice) Name() string {
	return d.name
}

func (d *windowsDevice) Configure(cfg *DeviceConfig) error {
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

func (d *windowsDevice) AddIP(ip string) error {
	// Parse CIDR
	_, ipNet, err := net.ParseCIDR(ip)
	if err != nil {
		return fmt.Errorf("invalid IP: %w", err)
	}

	// On Windows, use netsh to add IP address
	ones, _ := ipNet.Mask.Size()
	cmd := exec.Command("netsh", "interface", "ip", "add", "address",
		d.name, ip[:len(ip)-len(fmt.Sprintf("/%d", ones))],
		ipNetMaskWindows(ipNet))
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "already") {
			return nil
		}
		return fmt.Errorf("failed to add IP: %s: %w", output, err)
	}
	return nil
}

func (d *windowsDevice) RemoveIP(ip string) error {
	_, ipNet, err := net.ParseCIDR(ip)
	if err != nil {
		return fmt.Errorf("invalid IP: %w", err)
	}

	ones, _ := ipNet.Mask.Size()
	cmd := exec.Command("netsh", "interface", "ip", "delete", "address",
		d.name, ip[:len(ip)-len(fmt.Sprintf("/%d", ones))])
	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "not found") {
			return nil
		}
		return fmt.Errorf("failed to remove IP: %s: %w", output, err)
	}
	return nil
}

func (d *windowsDevice) Up() error {
	cmd := exec.Command("netsh", "interface", "set", "interface", d.name, "admin=enable")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface up: %s: %w", output, err)
	}
	return nil
}

func (d *windowsDevice) Down() error {
	cmd := exec.Command("netsh", "interface", "set", "interface", d.name, "admin=disable")
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to bring interface down: %s: %w", output, err)
	}
	return nil
}

func (d *windowsDevice) IsUp() bool {
	iface, err := net.InterfaceByName(d.name)
	if err != nil {
		return false
	}
	return iface.Flags&net.FlagUp != 0
}

func (d *windowsDevice) GetConfig() (*DeviceConfig, error) {
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

func (d *windowsDevice) Close() error {
	return d.client.Close()
}

func ipNetMaskWindows(ipNet *net.IPNet) string {
	mask := ipNet.Mask
	if len(mask) == 4 {
		return fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
	}
	return mask.String()
}
