package wireguard

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/curve25519"
)

const (
	// KeySize is the size of a WireGuard key in bytes (Curve25519).
	KeySize = 32
)

// GenerateKeyPair generates a new WireGuard key pair.
func GenerateKeyPair() (*KeyPair, error) {
	// Generate private key
	privateKey := make([]byte, KeySize)
	if _, err := rand.Read(privateKey); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyGenerationFailed, err)
	}

	// Apply Curve25519 clamping
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	// Derive public key
	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrKeyGenerationFailed, err)
	}

	return &KeyPair{
		PrivateKey: EncodeKey(privateKey),
		PublicKey:  EncodeKey(publicKey),
	}, nil
}

// DerivePublicKey derives the public key from a private key.
func DerivePublicKey(privateKeyBase64 string) (string, error) {
	privateKey, err := DecodeKey(privateKeyBase64)
	if err != nil {
		return "", fmt.Errorf("invalid private key: %w", err)
	}

	publicKey, err := curve25519.X25519(privateKey, curve25519.Basepoint)
	if err != nil {
		return "", fmt.Errorf("failed to derive public key: %w", err)
	}

	return EncodeKey(publicKey), nil
}

// EncodeKey encodes a key to base64.
func EncodeKey(key []byte) string {
	return base64.StdEncoding.EncodeToString(key)
}

// DecodeKey decodes a base64-encoded key.
func DecodeKey(keyBase64 string) ([]byte, error) {
	key, err := base64.StdEncoding.DecodeString(keyBase64)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}
	if len(key) != KeySize {
		return nil, fmt.Errorf("invalid key size: expected %d, got %d", KeySize, len(key))
	}
	return key, nil
}

// ValidatePrivateKey validates a base64-encoded private key.
func ValidatePrivateKey(privateKeyBase64 string) error {
	key, err := DecodeKey(privateKeyBase64)
	if err != nil {
		return ErrInvalidPrivateKey
	}

	// Check Curve25519 clamping
	if key[0]&7 != 0 || key[31]&128 != 0 || key[31]&64 == 0 {
		return ErrInvalidPrivateKey
	}

	return nil
}

// ValidatePublicKey validates a base64-encoded public key.
func ValidatePublicKey(publicKeyBase64 string) error {
	_, err := DecodeKey(publicKeyBase64)
	if err != nil {
		return ErrInvalidPublicKey
	}
	return nil
}

// GeneratePresharedKey generates a random pre-shared key.
func GeneratePresharedKey() (string, error) {
	psk := make([]byte, KeySize)
	if _, err := rand.Read(psk); err != nil {
		return "", fmt.Errorf("%w: preshared key: %v", ErrKeyGenerationFailed, err)
	}
	return EncodeKey(psk), nil
}
