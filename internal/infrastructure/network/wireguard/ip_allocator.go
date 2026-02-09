package wireguard

import (
	"encoding/binary"
	"fmt"
	"net"
	"sync"
)

// IPAllocator manages IP address allocation within a subnet.
type IPAllocator struct {
	mu        sync.Mutex
	subnet    *net.IPNet
	allocated map[string]string // IP -> peerID
	peerToIP  map[string]string // peerID -> IP
	nextIndex uint32
}

// NewIPAllocator creates a new IP allocator for the given subnet.
func NewIPAllocator(subnetCIDR string) (*IPAllocator, error) {
	_, subnet, err := net.ParseCIDR(subnetCIDR)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidSubnet, err)
	}

	return &IPAllocator{
		subnet:    subnet,
		allocated: make(map[string]string),
		peerToIP:  make(map[string]string),
		nextIndex: 1, // Start at .1
	}, nil
}

// Allocate allocates an IP address for the given peer ID.
// Returns the allocated IP with CIDR notation (e.g., "10.100.0.1/24").
func (a *IPAllocator) Allocate(peerID string) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Check if peer already has an IP
	if ip, ok := a.peerToIP[peerID]; ok {
		return ip, nil
	}

	// Find next available IP
	ip, err := a.findNextAvailable()
	if err != nil {
		return "", err
	}

	// Get prefix length from subnet
	ones, _ := a.subnet.Mask.Size()
	ipWithCIDR := fmt.Sprintf("%s/%d", ip.String(), ones)

	a.allocated[ip.String()] = peerID
	a.peerToIP[peerID] = ipWithCIDR

	return ipWithCIDR, nil
}

// AllocateSpecific allocates a specific IP for the given peer ID.
func (a *IPAllocator) AllocateSpecific(peerID, ipCIDR string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	ip, _, err := net.ParseCIDR(ipCIDR)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidLocalIP, err)
	}

	if !a.subnet.Contains(ip) {
		return fmt.Errorf("IP %s is not within subnet %s", ip, a.subnet)
	}

	ipStr := ip.String()
	if existingPeer, ok := a.allocated[ipStr]; ok {
		if existingPeer != peerID {
			return ErrIPAlreadyAllocated
		}
		return nil // Already allocated to this peer
	}

	a.allocated[ipStr] = peerID
	a.peerToIP[peerID] = ipCIDR

	return nil
}

// Release releases the IP allocated to the given peer ID.
func (a *IPAllocator) Release(peerID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	ipCIDR, ok := a.peerToIP[peerID]
	if !ok {
		return ErrIPNotAllocated
	}

	ip, _, _ := net.ParseCIDR(ipCIDR)
	delete(a.allocated, ip.String())
	delete(a.peerToIP, peerID)

	return nil
}

// GetIP returns the IP allocated to the given peer ID.
func (a *IPAllocator) GetIP(peerID string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	ip, ok := a.peerToIP[peerID]
	return ip, ok
}

// GetPeer returns the peer ID for the given IP.
func (a *IPAllocator) GetPeer(ip string) (string, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Strip CIDR notation if present
	if parsedIP, _, err := net.ParseCIDR(ip); err == nil {
		ip = parsedIP.String()
	}

	peerID, ok := a.allocated[ip]
	return peerID, ok
}

// IsAllocated checks if an IP is allocated.
func (a *IPAllocator) IsAllocated(ip string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	// Strip CIDR notation if present
	if parsedIP, _, err := net.ParseCIDR(ip); err == nil {
		ip = parsedIP.String()
	}

	_, ok := a.allocated[ip]
	return ok
}

// AllocatedCount returns the number of allocated IPs.
func (a *IPAllocator) AllocatedCount() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	return len(a.allocated)
}

// Subnet returns the subnet CIDR.
func (a *IPAllocator) Subnet() string {
	return a.subnet.String()
}

// findNextAvailable finds the next available IP in the subnet.
func (a *IPAllocator) findNextAvailable() (net.IP, error) {
	ones, bits := a.subnet.Mask.Size()
	maxHosts := uint32(1<<(bits-ones)) - 2 // Exclude network and broadcast

	// Get base IP as uint32
	baseIP := binary.BigEndian.Uint32(a.subnet.IP.To4())

	// Try to find an available IP
	for i := uint32(0); i < maxHosts; i++ {
		idx := (a.nextIndex + i - 1) % maxHosts + 1 // 1 to maxHosts
		candidateIP := make(net.IP, 4)
		binary.BigEndian.PutUint32(candidateIP, baseIP+idx)

		if _, ok := a.allocated[candidateIP.String()]; !ok {
			a.nextIndex = idx + 1
			if a.nextIndex > maxHosts {
				a.nextIndex = 1
			}
			return candidateIP, nil
		}
	}

	return nil, ErrSubnetExhausted
}

// ListAllocations returns all current allocations.
func (a *IPAllocator) ListAllocations() map[string]string {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := make(map[string]string)
	for peerID, ip := range a.peerToIP {
		result[peerID] = ip
	}
	return result
}
