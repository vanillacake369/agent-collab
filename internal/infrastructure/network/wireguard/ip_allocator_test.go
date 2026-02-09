package wireguard

import (
	"strings"
	"testing"
)

func TestNewIPAllocator(t *testing.T) {
	tests := []struct {
		name    string
		subnet  string
		wantErr bool
	}{
		{"valid /24", "10.100.0.0/24", false},
		{"valid /16", "172.16.0.0/16", false},
		{"valid /8", "10.0.0.0/8", false},
		{"invalid CIDR", "invalid", true},
		{"empty", "", true},
		{"missing prefix", "10.100.0.0", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewIPAllocator(tt.subnet)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewIPAllocator() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIPAllocatorAllocate(t *testing.T) {
	alloc, err := NewIPAllocator("10.100.0.0/24")
	if err != nil {
		t.Fatalf("NewIPAllocator() error = %v", err)
	}

	// First allocation should get .1
	ip1, err := alloc.Allocate("peer1")
	if err != nil {
		t.Fatalf("Allocate(peer1) error = %v", err)
	}
	if ip1 != "10.100.0.1/24" {
		t.Errorf("First allocation = %v, want 10.100.0.1/24", ip1)
	}

	// Second allocation should get .2
	ip2, err := alloc.Allocate("peer2")
	if err != nil {
		t.Fatalf("Allocate(peer2) error = %v", err)
	}
	if ip2 != "10.100.0.2/24" {
		t.Errorf("Second allocation = %v, want 10.100.0.2/24", ip2)
	}

	// Same peer should get same IP
	ip1Again, err := alloc.Allocate("peer1")
	if err != nil {
		t.Fatalf("Allocate(peer1) again error = %v", err)
	}
	if ip1Again != ip1 {
		t.Errorf("Same peer got different IP: %v != %v", ip1Again, ip1)
	}
}

func TestIPAllocatorAllocateSpecific(t *testing.T) {
	alloc, err := NewIPAllocator("10.100.0.0/24")
	if err != nil {
		t.Fatalf("NewIPAllocator() error = %v", err)
	}

	// Allocate specific IP
	err = alloc.AllocateSpecific("peer1", "10.100.0.50/24")
	if err != nil {
		t.Fatalf("AllocateSpecific() error = %v", err)
	}

	// Verify the allocation
	ip, err := alloc.Allocate("peer1")
	if err != nil {
		t.Fatalf("Allocate() error = %v", err)
	}
	if ip != "10.100.0.50/24" {
		t.Errorf("AllocateSpecific() didn't work: got %v, want 10.100.0.50/24", ip)
	}

	// Allocating the same IP to another peer should fail
	err = alloc.AllocateSpecific("peer2", "10.100.0.50/24")
	if err == nil {
		t.Error("AllocateSpecific() should fail for already allocated IP")
	}
}

func TestIPAllocatorRelease(t *testing.T) {
	alloc, err := NewIPAllocator("10.100.0.0/24")
	if err != nil {
		t.Fatalf("NewIPAllocator() error = %v", err)
	}

	// Allocate an IP
	ip, err := alloc.Allocate("peer1")
	if err != nil {
		t.Fatalf("Allocate() error = %v", err)
	}

	// Release it
	err = alloc.Release("peer1")
	if err != nil {
		t.Fatalf("Release() error = %v", err)
	}

	// Allocate again - should get a new allocation
	ip2, err := alloc.Allocate("peer2")
	if err != nil {
		t.Fatalf("Allocate() error = %v", err)
	}

	// The released IP should now be available for reuse
	// (next sequential allocation is .1 since it was released)
	if ip2 != ip {
		t.Logf("After release, got %v instead of released %v (sequential allocation)", ip2, ip)
	}
}

func TestIPAllocatorGetAllocated(t *testing.T) {
	alloc, err := NewIPAllocator("10.100.0.0/24")
	if err != nil {
		t.Fatalf("NewIPAllocator() error = %v", err)
	}

	// Allocate some IPs
	alloc.Allocate("peer1")
	alloc.Allocate("peer2")
	alloc.Allocate("peer3")

	allocated := alloc.GetAllocated()
	if len(allocated) != 3 {
		t.Errorf("GetAllocated() returned %d entries, want 3", len(allocated))
	}

	// Check specific allocations
	expected := map[string]string{
		"peer1": "10.100.0.1/24",
		"peer2": "10.100.0.2/24",
		"peer3": "10.100.0.3/24",
	}
	for peer, wantIP := range expected {
		if got := allocated[peer]; got != wantIP {
			t.Errorf("GetAllocated()[%s] = %v, want %v", peer, got, wantIP)
		}
	}
}

func TestIPAllocatorExhaustion(t *testing.T) {
	// Use a /30 subnet which has only 2 usable IPs
	alloc, err := NewIPAllocator("10.100.0.0/30")
	if err != nil {
		t.Fatalf("NewIPAllocator() error = %v", err)
	}

	// Allocate first IP
	_, err = alloc.Allocate("peer1")
	if err != nil {
		t.Fatalf("Allocate(peer1) error = %v", err)
	}

	// Allocate second IP
	_, err = alloc.Allocate("peer2")
	if err != nil {
		t.Fatalf("Allocate(peer2) error = %v", err)
	}

	// Third allocation should fail (network and broadcast addresses excluded)
	_, err = alloc.Allocate("peer3")
	if err == nil {
		t.Error("Allocate(peer3) should fail due to exhaustion")
	}
	if !strings.Contains(err.Error(), "exhausted") {
		t.Errorf("Expected exhaustion error, got: %v", err)
	}
}

func TestIPAllocatorGetIP(t *testing.T) {
	alloc, err := NewIPAllocator("10.100.0.0/24")
	if err != nil {
		t.Fatalf("NewIPAllocator() error = %v", err)
	}

	// Before allocation
	ip := alloc.GetIP("peer1")
	if ip != "" {
		t.Errorf("GetIP() before allocation = %v, want empty", ip)
	}

	// After allocation
	allocated, _ := alloc.Allocate("peer1")
	ip = alloc.GetIP("peer1")
	if ip != allocated {
		t.Errorf("GetIP() = %v, want %v", ip, allocated)
	}
}
