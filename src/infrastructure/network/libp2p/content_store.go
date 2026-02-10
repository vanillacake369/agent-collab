package libp2p

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ContentID is a content-addressed identifier (CID)
type ContentID string

// ContentReference is a reference to content stored elsewhere
type ContentReference struct {
	CID       ContentID `json:"cid"`
	Size      int       `json:"size"`
	MimeType  string    `json:"mime_type,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by,omitempty"`
}

// ContentStore stores content by CID for deduplication
type ContentStore struct {
	mu       sync.RWMutex
	content  map[ContentID][]byte
	metadata map[ContentID]*ContentMetadata
	maxSize  int64 // Maximum total storage size
	curSize  int64 // Current storage size
	ttl      time.Duration
}

// ContentMetadata holds metadata about stored content
type ContentMetadata struct {
	Size        int
	MimeType    string
	CreatedAt   time.Time
	LastAccess  time.Time
	AccessCount int64
}

// ContentStoreConfig configures the content store
type ContentStoreConfig struct {
	MaxSize int64         // Maximum storage size in bytes (default: 100MB)
	TTL     time.Duration // Content TTL (default: 1 hour)
}

// DefaultContentStoreConfig returns the default configuration
func DefaultContentStoreConfig() ContentStoreConfig {
	return ContentStoreConfig{
		MaxSize: 100 * 1024 * 1024, // 100MB
		TTL:     1 * time.Hour,
	}
}

// NewContentStore creates a new content store
func NewContentStore(config ContentStoreConfig) *ContentStore {
	if config.MaxSize == 0 {
		config.MaxSize = DefaultContentStoreConfig().MaxSize
	}
	if config.TTL == 0 {
		config.TTL = DefaultContentStoreConfig().TTL
	}

	cs := &ContentStore{
		content:  make(map[ContentID][]byte),
		metadata: make(map[ContentID]*ContentMetadata),
		maxSize:  config.MaxSize,
		ttl:      config.TTL,
	}

	// Start cleanup goroutine
	go cs.cleanupLoop()

	return cs
}

// Put stores content and returns its CID
func (cs *ContentStore) Put(data []byte) (ContentID, error) {
	cid := computeCID(data)

	cs.mu.Lock()
	defer cs.mu.Unlock()

	// Check if already exists
	if _, exists := cs.content[cid]; exists {
		// Update access time
		if meta := cs.metadata[cid]; meta != nil {
			meta.LastAccess = time.Now()
			meta.AccessCount++
		}
		return cid, nil
	}

	// Check size limit
	if cs.curSize+int64(len(data)) > cs.maxSize {
		// Evict oldest content
		cs.evictLocked(int64(len(data)))
	}

	// Store content
	cs.content[cid] = data
	cs.metadata[cid] = &ContentMetadata{
		Size:        len(data),
		CreatedAt:   time.Now(),
		LastAccess:  time.Now(),
		AccessCount: 1,
	}
	cs.curSize += int64(len(data))

	return cid, nil
}

// PutWithMimeType stores content with mime type
func (cs *ContentStore) PutWithMimeType(data []byte, mimeType string) (ContentID, error) {
	cid, err := cs.Put(data)
	if err != nil {
		return "", err
	}

	cs.mu.Lock()
	if meta := cs.metadata[cid]; meta != nil {
		meta.MimeType = mimeType
	}
	cs.mu.Unlock()

	return cid, nil
}

// Get retrieves content by CID
func (cs *ContentStore) Get(cid ContentID) ([]byte, error) {
	cs.mu.RLock()
	data, exists := cs.content[cid]
	cs.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("content not found: %s", cid)
	}

	// Update access time
	cs.mu.Lock()
	if meta := cs.metadata[cid]; meta != nil {
		meta.LastAccess = time.Now()
		meta.AccessCount++
	}
	cs.mu.Unlock()

	return data, nil
}

// Has checks if content exists
func (cs *ContentStore) Has(cid ContentID) bool {
	cs.mu.RLock()
	_, exists := cs.content[cid]
	cs.mu.RUnlock()
	return exists
}

// GetMetadata returns metadata for a CID
func (cs *ContentStore) GetMetadata(cid ContentID) *ContentMetadata {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.metadata[cid]
}

// Delete removes content
func (cs *ContentStore) Delete(cid ContentID) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	data, exists := cs.content[cid]
	if !exists {
		return nil
	}

	delete(cs.content, cid)
	delete(cs.metadata, cid)
	cs.curSize -= int64(len(data))

	return nil
}

// List returns all CIDs
func (cs *ContentStore) List() []ContentID {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	result := make([]ContentID, 0, len(cs.content))
	for cid := range cs.content {
		result = append(result, cid)
	}
	return result
}

// Stats returns storage statistics
type ContentStoreStats struct {
	ItemCount  int     `json:"item_count"`
	TotalSize  int64   `json:"total_size"`
	MaxSize    int64   `json:"max_size"`
	UsageRatio float64 `json:"usage_ratio"`
}

func (cs *ContentStore) Stats() ContentStoreStats {
	cs.mu.RLock()
	defer cs.mu.RUnlock()

	return ContentStoreStats{
		ItemCount:  len(cs.content),
		TotalSize:  cs.curSize,
		MaxSize:    cs.maxSize,
		UsageRatio: float64(cs.curSize) / float64(cs.maxSize),
	}
}

// CreateReference creates a reference for content
func (cs *ContentStore) CreateReference(cid ContentID, createdBy string) *ContentReference {
	cs.mu.RLock()
	meta := cs.metadata[cid]
	cs.mu.RUnlock()

	if meta == nil {
		return nil
	}

	return &ContentReference{
		CID:       cid,
		Size:      meta.Size,
		MimeType:  meta.MimeType,
		CreatedAt: meta.CreatedAt,
		CreatedBy: createdBy,
	}
}

// evictLocked evicts content to free space (caller must hold lock)
func (cs *ContentStore) evictLocked(needed int64) {
	// Simple LRU eviction
	type cidAccess struct {
		cid        ContentID
		lastAccess time.Time
		size       int
	}

	var items []cidAccess
	for cid, meta := range cs.metadata {
		items = append(items, cidAccess{
			cid:        cid,
			lastAccess: meta.LastAccess,
			size:       meta.Size,
		})
	}

	// Sort by last access (oldest first)
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[i].lastAccess.After(items[j].lastAccess) {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	// Evict until we have enough space
	var freed int64
	for _, item := range items {
		if cs.curSize-freed+needed <= cs.maxSize {
			break
		}
		delete(cs.content, item.cid)
		delete(cs.metadata, item.cid)
		freed += int64(item.size)
	}
	cs.curSize -= freed
}

// cleanupLoop removes expired content
func (cs *ContentStore) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		cs.cleanup()
	}
}

func (cs *ContentStore) cleanup() {
	cs.mu.Lock()
	defer cs.mu.Unlock()

	now := time.Now()
	for cid, meta := range cs.metadata {
		if now.Sub(meta.LastAccess) > cs.ttl {
			if data, exists := cs.content[cid]; exists {
				cs.curSize -= int64(len(data))
			}
			delete(cs.content, cid)
			delete(cs.metadata, cid)
		}
	}
}

// computeCID computes a CID for content
func computeCID(data []byte) ContentID {
	hash := sha256.Sum256(data)
	return ContentID("sha256-" + hex.EncodeToString(hash[:]))
}

// ValidateCID checks if data matches the CID
func ValidateCID(cid ContentID, data []byte) bool {
	computed := computeCID(data)
	return computed == cid
}

// CIDFromString parses a CID string
func CIDFromString(s string) ContentID {
	return ContentID(s)
}

// String returns the string representation
func (c ContentID) String() string {
	return string(c)
}

// ContentAddressedMessage wraps a message with CID reference for large payloads
type ContentAddressedMessage struct {
	Type      string            `json:"type"`
	Reference *ContentReference `json:"reference,omitempty"`
	Inline    []byte            `json:"inline,omitempty"`
}

// ContentThreshold is the size above which content is stored by reference
const ContentThreshold = 4 * 1024 // 4KB

// WrapContent wraps content, using CID reference for large content
func (cs *ContentStore) WrapContent(data []byte, createdBy string) (*ContentAddressedMessage, error) {
	if len(data) <= ContentThreshold {
		return &ContentAddressedMessage{
			Type:   "inline",
			Inline: data,
		}, nil
	}

	cid, err := cs.Put(data)
	if err != nil {
		return nil, err
	}

	return &ContentAddressedMessage{
		Type:      "reference",
		Reference: cs.CreateReference(cid, createdBy),
	}, nil
}

// UnwrapContent unwraps content, fetching by CID if needed
func (cs *ContentStore) UnwrapContent(msg *ContentAddressedMessage) ([]byte, error) {
	if msg.Type == "inline" {
		return msg.Inline, nil
	}

	if msg.Reference == nil {
		return nil, fmt.Errorf("missing content reference")
	}

	return cs.Get(msg.Reference.CID)
}
