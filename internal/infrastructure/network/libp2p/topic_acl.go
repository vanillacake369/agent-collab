package libp2p

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

var (
	// ErrAccessDenied is returned when a peer is not authorized
	ErrAccessDenied = errors.New("access denied")
	// ErrInvalidKey is returned when the encryption key is invalid
	ErrInvalidKey = errors.New("invalid encryption key")
	// ErrDecryptionFailed is returned when decryption fails
	ErrDecryptionFailed = errors.New("decryption failed")
)

// TopicACL represents access control for a topic
type TopicACL struct {
	mu sync.RWMutex

	Topic     string               `json:"topic"`
	AllowList map[peer.ID]struct{} `json:"-"` // Allowed peers (empty = all allowed)
	DenyList  map[peer.ID]struct{} `json:"-"` // Denied peers (takes precedence)

	// Encryption
	EncryptionEnabled bool   `json:"encryption_enabled"`
	encryptionKey     []byte // AES-256 key (32 bytes)

	// Metadata
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	CreatedBy peer.ID   `json:"created_by"`
}

// ACLConfig configures an ACL
type ACLConfig struct {
	Topic             string
	AllowList         []peer.ID
	DenyList          []peer.ID
	EncryptionEnabled bool
	EncryptionKey     []byte // Must be 32 bytes for AES-256
	CreatedBy         peer.ID
}

// NewTopicACL creates a new topic ACL
func NewTopicACL(config ACLConfig) (*TopicACL, error) {
	acl := &TopicACL{
		Topic:             config.Topic,
		AllowList:         make(map[peer.ID]struct{}),
		DenyList:          make(map[peer.ID]struct{}),
		EncryptionEnabled: config.EncryptionEnabled,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		CreatedBy:         config.CreatedBy,
	}

	// Populate allow list
	for _, p := range config.AllowList {
		acl.AllowList[p] = struct{}{}
	}

	// Populate deny list
	for _, p := range config.DenyList {
		acl.DenyList[p] = struct{}{}
	}

	// Set encryption key
	if config.EncryptionEnabled {
		if len(config.EncryptionKey) != 32 {
			return nil, ErrInvalidKey
		}
		acl.encryptionKey = config.EncryptionKey
	}

	return acl, nil
}

// CanSubscribe checks if a peer can subscribe to the topic
func (acl *TopicACL) CanSubscribe(p peer.ID) bool {
	acl.mu.RLock()
	defer acl.mu.RUnlock()

	// Deny list takes precedence
	if _, denied := acl.DenyList[p]; denied {
		return false
	}

	// If allow list is empty, all non-denied peers are allowed
	if len(acl.AllowList) == 0 {
		return true
	}

	// Otherwise, peer must be in allow list
	_, allowed := acl.AllowList[p]
	return allowed
}

// CanPublish checks if a peer can publish to the topic
func (acl *TopicACL) CanPublish(p peer.ID) bool {
	// For now, publish permissions are the same as subscribe
	return acl.CanSubscribe(p)
}

// AddToAllowList adds a peer to the allow list
func (acl *TopicACL) AddToAllowList(p peer.ID) {
	acl.mu.Lock()
	defer acl.mu.Unlock()
	acl.AllowList[p] = struct{}{}
	acl.UpdatedAt = time.Now()
}

// RemoveFromAllowList removes a peer from the allow list
func (acl *TopicACL) RemoveFromAllowList(p peer.ID) {
	acl.mu.Lock()
	defer acl.mu.Unlock()
	delete(acl.AllowList, p)
	acl.UpdatedAt = time.Now()
}

// AddToDenyList adds a peer to the deny list
func (acl *TopicACL) AddToDenyList(p peer.ID) {
	acl.mu.Lock()
	defer acl.mu.Unlock()
	acl.DenyList[p] = struct{}{}
	acl.UpdatedAt = time.Now()
}

// RemoveFromDenyList removes a peer from the deny list
func (acl *TopicACL) RemoveFromDenyList(p peer.ID) {
	acl.mu.Lock()
	defer acl.mu.Unlock()
	delete(acl.DenyList, p)
	acl.UpdatedAt = time.Now()
}

// GetAllowList returns the list of allowed peers
func (acl *TopicACL) GetAllowList() []peer.ID {
	acl.mu.RLock()
	defer acl.mu.RUnlock()

	result := make([]peer.ID, 0, len(acl.AllowList))
	for p := range acl.AllowList {
		result = append(result, p)
	}
	return result
}

// GetDenyList returns the list of denied peers
func (acl *TopicACL) GetDenyList() []peer.ID {
	acl.mu.RLock()
	defer acl.mu.RUnlock()

	result := make([]peer.ID, 0, len(acl.DenyList))
	for p := range acl.DenyList {
		result = append(result, p)
	}
	return result
}

// Encrypt encrypts a message using AES-GCM
func (acl *TopicACL) Encrypt(plaintext []byte) ([]byte, error) {
	acl.mu.RLock()
	defer acl.mu.RUnlock()

	if !acl.EncryptionEnabled || acl.encryptionKey == nil {
		return plaintext, nil // No encryption
	}

	block, err := aes.NewCipher(acl.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	// Encrypt and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts a message using AES-GCM
func (acl *TopicACL) Decrypt(ciphertext []byte) ([]byte, error) {
	acl.mu.RLock()
	defer acl.mu.RUnlock()

	if !acl.EncryptionEnabled || acl.encryptionKey == nil {
		return ciphertext, nil // No encryption
	}

	block, err := aes.NewCipher(acl.encryptionKey)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrDecryptionFailed
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// SetEncryptionKey sets or updates the encryption key
func (acl *TopicACL) SetEncryptionKey(key []byte) error {
	if len(key) != 32 {
		return ErrInvalidKey
	}

	acl.mu.Lock()
	defer acl.mu.Unlock()
	acl.encryptionKey = key
	acl.EncryptionEnabled = true
	acl.UpdatedAt = time.Now()
	return nil
}

// DisableEncryption disables encryption for this topic
func (acl *TopicACL) DisableEncryption() {
	acl.mu.Lock()
	defer acl.mu.Unlock()
	acl.EncryptionEnabled = false
	acl.encryptionKey = nil
	acl.UpdatedAt = time.Now()
}

// ACLManager manages ACLs for multiple topics
type ACLManager struct {
	mu   sync.RWMutex
	acls map[string]*TopicACL

	// Default policy
	defaultPolicy ACLPolicy

	// Callbacks
	onACLChange func(topic string, event ACLChangeEvent)
}

// ACLPolicy defines the default access policy
type ACLPolicy int

const (
	// PolicyAllowAll allows all peers by default
	PolicyAllowAll ACLPolicy = iota
	// PolicyDenyAll denies all peers by default (requires explicit allow)
	PolicyDenyAll
)

// ACLChangeEvent represents a change to an ACL
type ACLChangeEvent struct {
	Type      ACLChangeType
	Topic     string
	PeerID    peer.ID
	Timestamp time.Time
}

// ACLChangeType identifies the type of ACL change
type ACLChangeType int

const (
	EventACLCreated ACLChangeType = iota
	EventACLDeleted
	EventPeerAllowed
	EventPeerDenied
	EventPeerRemoved
	EventEncryptionChanged
)

// NewACLManager creates a new ACL manager
func NewACLManager(defaultPolicy ACLPolicy) *ACLManager {
	return &ACLManager{
		acls:          make(map[string]*TopicACL),
		defaultPolicy: defaultPolicy,
	}
}

// OnACLChange registers a callback for ACL changes
func (am *ACLManager) OnACLChange(fn func(topic string, event ACLChangeEvent)) {
	am.mu.Lock()
	am.onACLChange = fn
	am.mu.Unlock()
}

// CreateACL creates a new ACL for a topic
func (am *ACLManager) CreateACL(config ACLConfig) (*TopicACL, error) {
	acl, err := NewTopicACL(config)
	if err != nil {
		return nil, err
	}

	am.mu.Lock()
	am.acls[config.Topic] = acl
	callback := am.onACLChange
	am.mu.Unlock()

	if callback != nil {
		callback(config.Topic, ACLChangeEvent{
			Type:      EventACLCreated,
			Topic:     config.Topic,
			Timestamp: time.Now(),
		})
	}

	return acl, nil
}

// GetACL returns the ACL for a topic
func (am *ACLManager) GetACL(topic string) *TopicACL {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.acls[topic]
}

// DeleteACL removes an ACL for a topic
func (am *ACLManager) DeleteACL(topic string) {
	am.mu.Lock()
	delete(am.acls, topic)
	callback := am.onACLChange
	am.mu.Unlock()

	if callback != nil {
		callback(topic, ACLChangeEvent{
			Type:      EventACLDeleted,
			Topic:     topic,
			Timestamp: time.Now(),
		})
	}
}

// CanSubscribe checks if a peer can subscribe to a topic
func (am *ACLManager) CanSubscribe(topic string, p peer.ID) bool {
	am.mu.RLock()
	acl, exists := am.acls[topic]
	defaultPolicy := am.defaultPolicy
	am.mu.RUnlock()

	if !exists {
		// Use default policy
		return defaultPolicy == PolicyAllowAll
	}

	return acl.CanSubscribe(p)
}

// CanPublish checks if a peer can publish to a topic
func (am *ACLManager) CanPublish(topic string, p peer.ID) bool {
	am.mu.RLock()
	acl, exists := am.acls[topic]
	defaultPolicy := am.defaultPolicy
	am.mu.RUnlock()

	if !exists {
		return defaultPolicy == PolicyAllowAll
	}

	return acl.CanPublish(p)
}

// Encrypt encrypts a message for a topic
func (am *ACLManager) Encrypt(topic string, plaintext []byte) ([]byte, error) {
	am.mu.RLock()
	acl, exists := am.acls[topic]
	am.mu.RUnlock()

	if !exists {
		return plaintext, nil
	}

	return acl.Encrypt(plaintext)
}

// Decrypt decrypts a message from a topic
func (am *ACLManager) Decrypt(topic string, ciphertext []byte) ([]byte, error) {
	am.mu.RLock()
	acl, exists := am.acls[topic]
	am.mu.RUnlock()

	if !exists {
		return ciphertext, nil
	}

	return acl.Decrypt(ciphertext)
}

// AllowPeer allows a peer to access a topic
func (am *ACLManager) AllowPeer(topic string, p peer.ID) error {
	am.mu.RLock()
	acl, exists := am.acls[topic]
	callback := am.onACLChange
	am.mu.RUnlock()

	if !exists {
		return errors.New("topic ACL not found")
	}

	acl.AddToAllowList(p)

	if callback != nil {
		callback(topic, ACLChangeEvent{
			Type:      EventPeerAllowed,
			Topic:     topic,
			PeerID:    p,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// DenyPeer denies a peer access to a topic
func (am *ACLManager) DenyPeer(topic string, p peer.ID) error {
	am.mu.RLock()
	acl, exists := am.acls[topic]
	callback := am.onACLChange
	am.mu.RUnlock()

	if !exists {
		return errors.New("topic ACL not found")
	}

	acl.AddToDenyList(p)

	if callback != nil {
		callback(topic, ACLChangeEvent{
			Type:      EventPeerDenied,
			Topic:     topic,
			PeerID:    p,
			Timestamp: time.Now(),
		})
	}

	return nil
}

// ListTopics returns all topics with ACLs
func (am *ACLManager) ListTopics() []string {
	am.mu.RLock()
	defer am.mu.RUnlock()

	topics := make([]string, 0, len(am.acls))
	for topic := range am.acls {
		topics = append(topics, topic)
	}
	return topics
}

// Stats returns ACL statistics
func (am *ACLManager) Stats() ACLStats {
	am.mu.RLock()
	defer am.mu.RUnlock()

	stats := ACLStats{
		TotalTopics: len(am.acls),
	}

	for _, acl := range am.acls {
		acl.mu.RLock()
		if acl.EncryptionEnabled {
			stats.EncryptedTopics++
		}
		stats.TotalAllowListEntries += len(acl.AllowList)
		stats.TotalDenyListEntries += len(acl.DenyList)
		acl.mu.RUnlock()
	}

	return stats
}

// ACLStats holds ACL statistics
type ACLStats struct {
	TotalTopics           int `json:"total_topics"`
	EncryptedTopics       int `json:"encrypted_topics"`
	TotalAllowListEntries int `json:"total_allow_list_entries"`
	TotalDenyListEntries  int `json:"total_deny_list_entries"`
}

// GenerateEncryptionKey generates a random 32-byte AES-256 key
func GenerateEncryptionKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	return key, nil
}
