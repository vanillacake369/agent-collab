package platform

import (
	"net"
	"testing"
)

func TestMockPlatform(t *testing.T) {
	p := NewMockPlatform()

	if p.Name() != "mock" {
		t.Errorf("Name() = %s, want mock", p.Name())
	}

	if !p.IsSupported() {
		t.Error("IsSupported() = false, want true")
	}

	if p.RequiresRoot() {
		t.Error("RequiresRoot() = true, want false for mock")
	}
}

func TestMockPlatformCreateDeleteInterface(t *testing.T) {
	p := NewMockPlatform()

	// Create interface
	device, err := p.CreateInterface("wg-test")
	if err != nil {
		t.Fatalf("CreateInterface() error = %v", err)
	}

	if device.Name() != "wg-test" {
		t.Errorf("Device.Name() = %s, want wg-test", device.Name())
	}

	// Delete interface
	err = p.DeleteInterface("wg-test")
	if err != nil {
		t.Errorf("DeleteInterface() error = %v", err)
	}
}

func TestMockPlatformGetExternalIP(t *testing.T) {
	p := NewMockPlatform()

	ip, err := p.GetExternalIP()
	if err != nil {
		t.Fatalf("GetExternalIP() error = %v", err)
	}

	// Should return a valid IP
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		t.Errorf("GetExternalIP() returned invalid IP: %s", ip)
	}
}

func TestMockDeviceConfigure(t *testing.T) {
	p := NewMockPlatform()

	device, err := p.CreateInterface("wg-test")
	if err != nil {
		t.Fatalf("CreateInterface() error = %v", err)
	}
	defer device.Close()

	// Create configuration
	privateKey := make([]byte, 32)
	for i := range privateKey {
		privateKey[i] = byte(i)
	}

	cfg := &DeviceConfig{
		PrivateKey: privateKey,
		ListenPort: 51820,
		Peers: []PeerConfig{
			{
				PublicKey:                   make([]byte, 32),
				AllowedIPs:                  []net.IPNet{{IP: net.ParseIP("10.100.0.2"), Mask: net.CIDRMask(32, 32)}},
				PersistentKeepaliveInterval: 25,
			},
		},
	}

	err = device.Configure(cfg)
	if err != nil {
		t.Fatalf("Configure() error = %v", err)
	}

	// Get config back
	gotCfg, err := device.GetConfig()
	if err != nil {
		t.Fatalf("GetConfig() error = %v", err)
	}

	if gotCfg.ListenPort != cfg.ListenPort {
		t.Errorf("GetConfig().ListenPort = %d, want %d", gotCfg.ListenPort, cfg.ListenPort)
	}
}

func TestMockDeviceUpDown(t *testing.T) {
	p := NewMockPlatform()

	device, err := p.CreateInterface("wg-test")
	if err != nil {
		t.Fatalf("CreateInterface() error = %v", err)
	}
	defer device.Close()

	// Initially down
	if device.IsUp() {
		t.Error("Device should initially be down")
	}

	// Bring up
	err = device.Up()
	if err != nil {
		t.Fatalf("Up() error = %v", err)
	}

	if !device.IsUp() {
		t.Error("Device should be up after Up()")
	}

	// Bring down
	err = device.Down()
	if err != nil {
		t.Fatalf("Down() error = %v", err)
	}

	if device.IsUp() {
		t.Error("Device should be down after Down()")
	}
}

func TestMockDeviceAddRemoveIP(t *testing.T) {
	p := NewMockPlatform()

	device, err := p.CreateInterface("wg-test")
	if err != nil {
		t.Fatalf("CreateInterface() error = %v", err)
	}
	defer device.Close()

	// Add IP
	err = device.AddIP("10.100.0.1/24")
	if err != nil {
		t.Fatalf("AddIP() error = %v", err)
	}

	// Remove IP
	err = device.RemoveIP("10.100.0.1/24")
	if err != nil {
		t.Fatalf("RemoveIP() error = %v", err)
	}
}

func TestMockDeviceInvalidConfig(t *testing.T) {
	p := NewMockPlatform()

	device, err := p.CreateInterface("wg-test")
	if err != nil {
		t.Fatalf("CreateInterface() error = %v", err)
	}
	defer device.Close()

	// Configure with nil config
	err = device.Configure(nil)
	if err == nil {
		t.Error("Configure(nil) should return error")
	}

	// Configure with wrong key length
	cfg := &DeviceConfig{
		PrivateKey: []byte("short"),
		ListenPort: 51820,
	}
	err = device.Configure(cfg)
	if err == nil {
		t.Error("Configure() with invalid key should return error")
	}
}
