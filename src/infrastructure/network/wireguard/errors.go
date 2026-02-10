package wireguard

import "errors"

// ErrNotSupported indicates the platform doesn't support WireGuard.
var ErrNotSupported = errors.New("wireguard: platform not supported")

// ErrPermissionDenied indicates insufficient permissions.
var ErrPermissionDenied = errors.New("wireguard: permission denied (root required)")

// ErrInterfaceExists indicates the interface already exists.
var ErrInterfaceExists = errors.New("wireguard: interface already exists")

// ErrInterfaceNotFound indicates the interface was not found.
var ErrInterfaceNotFound = errors.New("wireguard: interface not found")

// ErrInterfaceNotRunning indicates the interface is not running.
var ErrInterfaceNotRunning = errors.New("wireguard: interface not running")

// ErrPeerNotFound indicates the peer was not found.
var ErrPeerNotFound = errors.New("wireguard: peer not found")

// ErrPeerExists indicates the peer already exists.
var ErrPeerExists = errors.New("wireguard: peer already exists")

// ErrInvalidPrivateKey indicates the private key is invalid.
var ErrInvalidPrivateKey = errors.New("wireguard: invalid private key")

// ErrInvalidPublicKey indicates the public key is invalid.
var ErrInvalidPublicKey = errors.New("wireguard: invalid public key")

// ErrInvalidPort indicates the port is invalid.
var ErrInvalidPort = errors.New("wireguard: invalid port (must be 0-65535)")

// ErrInvalidLocalIP indicates the local IP is invalid.
var ErrInvalidLocalIP = errors.New("wireguard: invalid local IP (must be valid CIDR)")

// ErrInvalidSubnet indicates the subnet is invalid.
var ErrInvalidSubnet = errors.New("wireguard: invalid subnet (must be valid CIDR)")

// ErrInvalidMTU indicates the MTU is invalid.
var ErrInvalidMTU = errors.New("wireguard: invalid MTU (must be 576-65535)")

// ErrSubnetExhausted indicates no more IPs available in the subnet.
var ErrSubnetExhausted = errors.New("wireguard: subnet exhausted, no available IPs")

// ErrIPAlreadyAllocated indicates the IP is already allocated.
var ErrIPAlreadyAllocated = errors.New("wireguard: IP already allocated")

// ErrIPNotAllocated indicates the IP is not allocated.
var ErrIPNotAllocated = errors.New("wireguard: IP not allocated")

// ErrNotInitialized indicates the manager is not initialized.
var ErrNotInitialized = errors.New("wireguard: manager not initialized")

// ErrAlreadyRunning indicates the manager is already running.
var ErrAlreadyRunning = errors.New("wireguard: manager already running")

// ErrInvalidEndpoint indicates the endpoint is invalid.
var ErrInvalidEndpoint = errors.New("wireguard: invalid endpoint")

// ErrKeyGenerationFailed indicates key generation failed.
var ErrKeyGenerationFailed = errors.New("wireguard: key generation failed")
