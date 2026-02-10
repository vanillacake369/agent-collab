package libp2p

import (
	"bytes"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestNewTopicACL(t *testing.T) {
	config := ACLConfig{
		Topic:     "/test/topic",
		AllowList: []peer.ID{"peer-a", "peer-b"},
		DenyList:  []peer.ID{"peer-c"},
		CreatedBy: "admin",
	}

	acl, err := NewTopicACL(config)
	if err != nil {
		t.Fatalf("NewTopicACL failed: %v", err)
	}

	if acl.Topic != "/test/topic" {
		t.Errorf("Topic = %s, want /test/topic", acl.Topic)
	}

	if len(acl.AllowList) != 2 {
		t.Errorf("AllowList size = %d, want 2", len(acl.AllowList))
	}

	if len(acl.DenyList) != 1 {
		t.Errorf("DenyList size = %d, want 1", len(acl.DenyList))
	}
}

func TestNewTopicACL_InvalidKey(t *testing.T) {
	config := ACLConfig{
		Topic:             "/test/topic",
		EncryptionEnabled: true,
		EncryptionKey:     []byte("short-key"), // Not 32 bytes
	}

	_, err := NewTopicACL(config)
	if err != ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey, got %v", err)
	}
}

func TestTopicACL_CanSubscribe(t *testing.T) {
	acl, _ := NewTopicACL(ACLConfig{
		Topic:     "/test/topic",
		AllowList: []peer.ID{"allowed-peer"},
		DenyList:  []peer.ID{"denied-peer"},
	})

	tests := []struct {
		peerID   peer.ID
		expected bool
	}{
		{"allowed-peer", true},
		{"denied-peer", false},
		{"unknown-peer", false}, // Not in allow list
	}

	for _, tt := range tests {
		if got := acl.CanSubscribe(tt.peerID); got != tt.expected {
			t.Errorf("CanSubscribe(%s) = %v, want %v", tt.peerID, got, tt.expected)
		}
	}
}

func TestTopicACL_CanSubscribe_EmptyAllowList(t *testing.T) {
	acl, _ := NewTopicACL(ACLConfig{
		Topic:    "/test/topic",
		DenyList: []peer.ID{"denied-peer"},
	})

	// Empty allow list means all non-denied peers are allowed
	if !acl.CanSubscribe("any-peer") {
		t.Error("Any peer should be allowed when allow list is empty")
	}

	if acl.CanSubscribe("denied-peer") {
		t.Error("Denied peer should not be allowed")
	}
}

func TestTopicACL_DenyListTakesPrecedence(t *testing.T) {
	acl, _ := NewTopicACL(ACLConfig{
		Topic:     "/test/topic",
		AllowList: []peer.ID{"peer-a"},
		DenyList:  []peer.ID{"peer-a"}, // Same peer in both lists
	})

	// Deny list should take precedence
	if acl.CanSubscribe("peer-a") {
		t.Error("Peer in deny list should be denied even if in allow list")
	}
}

func TestTopicACL_ModifyLists(t *testing.T) {
	acl, _ := NewTopicACL(ACLConfig{
		Topic: "/test/topic",
	})

	// Add to allow list
	acl.AddToAllowList("peer-a")
	if len(acl.GetAllowList()) != 1 {
		t.Error("Should have 1 peer in allow list")
	}

	// Add to deny list
	acl.AddToDenyList("peer-b")
	if len(acl.GetDenyList()) != 1 {
		t.Error("Should have 1 peer in deny list")
	}

	// Remove from allow list
	acl.RemoveFromAllowList("peer-a")
	if len(acl.GetAllowList()) != 0 {
		t.Error("Allow list should be empty")
	}

	// Remove from deny list
	acl.RemoveFromDenyList("peer-b")
	if len(acl.GetDenyList()) != 0 {
		t.Error("Deny list should be empty")
	}
}

func TestTopicACL_EncryptDecrypt(t *testing.T) {
	key, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("Failed to generate key: %v", err)
	}

	acl, err := NewTopicACL(ACLConfig{
		Topic:             "/test/topic",
		EncryptionEnabled: true,
		EncryptionKey:     key,
	})
	if err != nil {
		t.Fatalf("NewTopicACL failed: %v", err)
	}

	plaintext := []byte("Hello, World!")

	// Encrypt
	ciphertext, err := acl.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if bytes.Equal(ciphertext, plaintext) {
		t.Error("Ciphertext should be different from plaintext")
	}

	// Decrypt
	decrypted, err := acl.Decrypt(ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("Decrypted = %s, want %s", decrypted, plaintext)
	}
}

func TestTopicACL_NoEncryption(t *testing.T) {
	acl, _ := NewTopicACL(ACLConfig{
		Topic: "/test/topic",
	})

	plaintext := []byte("Hello, World!")

	// Should pass through unchanged
	result, err := acl.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if !bytes.Equal(result, plaintext) {
		t.Error("Without encryption, data should pass through unchanged")
	}
}

func TestTopicACL_SetEncryptionKey(t *testing.T) {
	acl, _ := NewTopicACL(ACLConfig{
		Topic: "/test/topic",
	})

	// Invalid key length
	err := acl.SetEncryptionKey([]byte("short"))
	if err != ErrInvalidKey {
		t.Errorf("Expected ErrInvalidKey, got %v", err)
	}

	// Valid key
	key, _ := GenerateEncryptionKey()
	err = acl.SetEncryptionKey(key)
	if err != nil {
		t.Errorf("SetEncryptionKey failed: %v", err)
	}

	if !acl.EncryptionEnabled {
		t.Error("EncryptionEnabled should be true")
	}
}

func TestTopicACL_DisableEncryption(t *testing.T) {
	key, _ := GenerateEncryptionKey()
	acl, _ := NewTopicACL(ACLConfig{
		Topic:             "/test/topic",
		EncryptionEnabled: true,
		EncryptionKey:     key,
	})

	acl.DisableEncryption()

	if acl.EncryptionEnabled {
		t.Error("EncryptionEnabled should be false")
	}
}

func TestACLManager_CreateGetDelete(t *testing.T) {
	am := NewACLManager(PolicyAllowAll)

	// Create ACL
	_, err := am.CreateACL(ACLConfig{
		Topic: "/test/topic",
	})
	if err != nil {
		t.Fatalf("CreateACL failed: %v", err)
	}

	// Get ACL
	acl := am.GetACL("/test/topic")
	if acl == nil {
		t.Fatal("GetACL returned nil")
	}

	// Delete ACL
	am.DeleteACL("/test/topic")

	acl = am.GetACL("/test/topic")
	if acl != nil {
		t.Error("ACL should be deleted")
	}
}

func TestACLManager_DefaultPolicy(t *testing.T) {
	t.Run("PolicyAllowAll", func(t *testing.T) {
		am := NewACLManager(PolicyAllowAll)

		// No ACL for this topic
		if !am.CanSubscribe("/unknown/topic", "any-peer") {
			t.Error("PolicyAllowAll should allow unknown topics")
		}
	})

	t.Run("PolicyDenyAll", func(t *testing.T) {
		am := NewACLManager(PolicyDenyAll)

		// No ACL for this topic
		if am.CanSubscribe("/unknown/topic", "any-peer") {
			t.Error("PolicyDenyAll should deny unknown topics")
		}
	})
}

func TestACLManager_AllowDenyPeer(t *testing.T) {
	am := NewACLManager(PolicyAllowAll)

	am.CreateACL(ACLConfig{
		Topic: "/test/topic",
	})

	// Allow a peer
	err := am.AllowPeer("/test/topic", "peer-a")
	if err != nil {
		t.Errorf("AllowPeer failed: %v", err)
	}

	// Deny a peer
	err = am.DenyPeer("/test/topic", "peer-b")
	if err != nil {
		t.Errorf("DenyPeer failed: %v", err)
	}

	// Check
	if !am.CanSubscribe("/test/topic", "peer-a") {
		t.Error("peer-a should be allowed")
	}
	if am.CanSubscribe("/test/topic", "peer-b") {
		t.Error("peer-b should be denied")
	}
}

func TestACLManager_EncryptDecrypt(t *testing.T) {
	am := NewACLManager(PolicyAllowAll)

	key, _ := GenerateEncryptionKey()
	am.CreateACL(ACLConfig{
		Topic:             "/encrypted/topic",
		EncryptionEnabled: true,
		EncryptionKey:     key,
	})

	plaintext := []byte("Secret message")

	ciphertext, err := am.Encrypt("/encrypted/topic", plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	decrypted, err := am.Decrypt("/encrypted/topic", ciphertext)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("Decrypted message doesn't match")
	}
}

func TestACLManager_ListTopics(t *testing.T) {
	am := NewACLManager(PolicyAllowAll)

	am.CreateACL(ACLConfig{Topic: "/topic/a"})
	am.CreateACL(ACLConfig{Topic: "/topic/b"})
	am.CreateACL(ACLConfig{Topic: "/topic/c"})

	topics := am.ListTopics()
	if len(topics) != 3 {
		t.Errorf("Expected 3 topics, got %d", len(topics))
	}
}

func TestACLManager_Stats(t *testing.T) {
	am := NewACLManager(PolicyAllowAll)

	key, _ := GenerateEncryptionKey()

	am.CreateACL(ACLConfig{
		Topic:     "/topic/a",
		AllowList: []peer.ID{"a", "b"},
		DenyList:  []peer.ID{"c"},
	})
	am.CreateACL(ACLConfig{
		Topic:             "/topic/b",
		EncryptionEnabled: true,
		EncryptionKey:     key,
	})

	stats := am.Stats()

	if stats.TotalTopics != 2 {
		t.Errorf("TotalTopics = %d, want 2", stats.TotalTopics)
	}
	if stats.EncryptedTopics != 1 {
		t.Errorf("EncryptedTopics = %d, want 1", stats.EncryptedTopics)
	}
	if stats.TotalAllowListEntries != 2 {
		t.Errorf("TotalAllowListEntries = %d, want 2", stats.TotalAllowListEntries)
	}
	if stats.TotalDenyListEntries != 1 {
		t.Errorf("TotalDenyListEntries = %d, want 1", stats.TotalDenyListEntries)
	}
}

func TestACLManager_OnACLChangeCallback(t *testing.T) {
	am := NewACLManager(PolicyAllowAll)

	var events []ACLChangeEvent

	am.OnACLChange(func(topic string, event ACLChangeEvent) {
		events = append(events, event)
	})

	am.CreateACL(ACLConfig{Topic: "/test/topic"})
	am.AllowPeer("/test/topic", "peer-a")
	am.DenyPeer("/test/topic", "peer-b")
	am.DeleteACL("/test/topic")

	if len(events) != 4 {
		t.Errorf("Expected 4 events, got %d", len(events))
	}

	expectedTypes := []ACLChangeType{
		EventACLCreated,
		EventPeerAllowed,
		EventPeerDenied,
		EventACLDeleted,
	}

	for i, expected := range expectedTypes {
		if events[i].Type != expected {
			t.Errorf("Event %d: type = %v, want %v", i, events[i].Type, expected)
		}
	}
}

func TestGenerateEncryptionKey(t *testing.T) {
	key1, err := GenerateEncryptionKey()
	if err != nil {
		t.Fatalf("GenerateEncryptionKey failed: %v", err)
	}

	if len(key1) != 32 {
		t.Errorf("Key length = %d, want 32", len(key1))
	}

	key2, _ := GenerateEncryptionKey()
	if bytes.Equal(key1, key2) {
		t.Error("Two generated keys should be different")
	}
}

func BenchmarkTopicACL_Encrypt(b *testing.B) {
	key, _ := GenerateEncryptionKey()
	acl, _ := NewTopicACL(ACLConfig{
		Topic:             "/test/topic",
		EncryptionEnabled: true,
		EncryptionKey:     key,
	})

	plaintext := make([]byte, 1024)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = acl.Encrypt(plaintext)
	}
}

func BenchmarkTopicACL_Decrypt(b *testing.B) {
	key, _ := GenerateEncryptionKey()
	acl, _ := NewTopicACL(ACLConfig{
		Topic:             "/test/topic",
		EncryptionEnabled: true,
		EncryptionKey:     key,
	})

	plaintext := make([]byte, 1024)
	ciphertext, _ := acl.Encrypt(plaintext)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = acl.Decrypt(ciphertext)
	}
}
