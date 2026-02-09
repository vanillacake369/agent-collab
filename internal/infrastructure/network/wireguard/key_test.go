package wireguard

import (
	"testing"
)

func TestGenerateKeyPair(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	if kp.PrivateKey == "" {
		t.Error("PrivateKey is empty")
	}
	if kp.PublicKey == "" {
		t.Error("PublicKey is empty")
	}

	// Keys should be base64-encoded 32-byte keys
	privBytes, err := DecodeKey(kp.PrivateKey)
	if err != nil {
		t.Errorf("DecodeKey(PrivateKey) error = %v", err)
	}
	if len(privBytes) != 32 {
		t.Errorf("PrivateKey length = %d, want 32", len(privBytes))
	}

	pubBytes, err := DecodeKey(kp.PublicKey)
	if err != nil {
		t.Errorf("DecodeKey(PublicKey) error = %v", err)
	}
	if len(pubBytes) != 32 {
		t.Errorf("PublicKey length = %d, want 32", len(pubBytes))
	}
}

func TestDerivePublicKey(t *testing.T) {
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	// Derive public key from private key
	derivedPub, err := DerivePublicKey(kp.PrivateKey)
	if err != nil {
		t.Fatalf("DerivePublicKey() error = %v", err)
	}

	if derivedPub != kp.PublicKey {
		t.Errorf("DerivePublicKey() = %v, want %v", derivedPub, kp.PublicKey)
	}
}

func TestEncodeDecodeKey(t *testing.T) {
	// Generate a key
	kp, err := GenerateKeyPair()
	if err != nil {
		t.Fatalf("GenerateKeyPair() error = %v", err)
	}

	// Decode and re-encode
	decoded, err := DecodeKey(kp.PrivateKey)
	if err != nil {
		t.Fatalf("DecodeKey() error = %v", err)
	}

	reEncoded := EncodeKey(decoded)
	if reEncoded != kp.PrivateKey {
		t.Errorf("Round-trip encode/decode failed: got %v, want %v", reEncoded, kp.PrivateKey)
	}
}

func TestKeyPairUniqueness(t *testing.T) {
	// Generate multiple key pairs and ensure they're unique
	keyPairs := make([]*KeyPair, 10)
	for i := 0; i < 10; i++ {
		kp, err := GenerateKeyPair()
		if err != nil {
			t.Fatalf("GenerateKeyPair() error = %v", err)
		}
		keyPairs[i] = kp
	}

	// Check all private keys are unique
	seen := make(map[string]bool)
	for i, kp := range keyPairs {
		if seen[kp.PrivateKey] {
			t.Errorf("Duplicate private key at index %d", i)
		}
		seen[kp.PrivateKey] = true
	}

	// Check all public keys are unique
	seen = make(map[string]bool)
	for i, kp := range keyPairs {
		if seen[kp.PublicKey] {
			t.Errorf("Duplicate public key at index %d", i)
		}
		seen[kp.PublicKey] = true
	}
}

func TestDerivePublicKeyInvalid(t *testing.T) {
	tests := []struct {
		name       string
		privateKey string
	}{
		{"empty", ""},
		{"invalid base64", "not-valid-base64!@#"},
		{"wrong length", "dG9vLXNob3J0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DerivePublicKey(tt.privateKey)
			if err == nil {
				t.Error("DerivePublicKey() expected error, got nil")
			}
		})
	}
}
