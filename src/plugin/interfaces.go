// Package plugin defines interfaces for pluggable infrastructure components.
// These interfaces allow the system to swap out implementations (e.g., libp2p, OpenAI)
// without changing the controller logic.
package plugin

import (
	"context"
	"time"
)

// Plugin is the base interface for all plugins.
type Plugin interface {
	// Name returns the plugin name.
	Name() string

	// Start initializes and starts the plugin.
	Start(ctx context.Context) error

	// Stop gracefully stops the plugin.
	Stop() error

	// Ready returns true if the plugin is ready to serve requests.
	Ready() bool
}

// PluginInfo contains metadata about a plugin.
type PluginInfo struct {
	// Name is the plugin name.
	Name string `json:"name"`

	// Version is the plugin version.
	Version string `json:"version"`

	// Description is a human-readable description.
	Description string `json:"description"`

	// Capabilities lists what the plugin can do.
	Capabilities []string `json:"capabilities"`
}

// NetworkPlugin provides peer-to-peer networking capabilities.
// It abstracts the underlying P2P implementation (e.g., libp2p).
type NetworkPlugin interface {
	Plugin

	// ID returns the local peer ID.
	ID() string

	// Addresses returns the multiaddresses where this peer is listening.
	Addresses() []string

	// Peers returns information about connected peers.
	Peers() []PeerInfo

	// PeerCount returns the number of connected peers.
	PeerCount() int

	// Publish broadcasts a message to a topic.
	Publish(ctx context.Context, topic string, data []byte) error

	// Subscribe subscribes to a topic and returns a channel for messages.
	Subscribe(ctx context.Context, topic string) (<-chan Message, error)

	// Unsubscribe unsubscribes from a topic.
	Unsubscribe(topic string) error

	// Connect connects to a specific peer.
	Connect(ctx context.Context, peerID string, addrs []string) error

	// Disconnect disconnects from a peer.
	Disconnect(peerID string) error

	// OnPeerConnect registers a callback for when a peer connects.
	OnPeerConnect(callback func(peerID string))

	// OnPeerDisconnect registers a callback for when a peer disconnects.
	OnPeerDisconnect(callback func(peerID string))
}

// PeerInfo contains information about a peer.
type PeerInfo struct {
	// ID is the peer's unique identifier.
	ID string `json:"id"`

	// Addresses are the multiaddresses where the peer can be reached.
	Addresses []string `json:"addresses"`

	// Latency is the round-trip latency to the peer.
	Latency time.Duration `json:"latency"`

	// Connected indicates if the peer is currently connected.
	Connected bool `json:"connected"`

	// ConnectedAt is when the peer connected.
	ConnectedAt *time.Time `json:"connectedAt,omitempty"`

	// Protocols are the protocols the peer supports.
	Protocols []string `json:"protocols,omitempty"`

	// Agent is the agent software string (e.g., "agent-collab/1.0.0").
	Agent string `json:"agent,omitempty"`
}

// Message represents a message received from the network.
type Message struct {
	// Topic is the topic the message was received on.
	Topic string

	// From is the peer ID of the sender.
	From string

	// Data is the message payload.
	Data []byte

	// ReceivedAt is when the message was received.
	ReceivedAt time.Time
}

// EmbeddingPlugin provides text embedding capabilities.
// It abstracts the underlying embedding provider (e.g., OpenAI, Anthropic).
type EmbeddingPlugin interface {
	Plugin

	// Embed generates an embedding for the given text.
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts.
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimension returns the embedding dimension.
	Dimension() int

	// Model returns the model name being used.
	Model() string

	// Provider returns the provider name (e.g., "openai", "anthropic").
	Provider() string
}

// StoragePlugin provides persistent storage capabilities.
// It abstracts the underlying storage implementation (e.g., BadgerDB).
type StoragePlugin interface {
	Plugin

	// Get retrieves a value by key.
	Get(ctx context.Context, key []byte) ([]byte, error)

	// Set stores a value.
	Set(ctx context.Context, key, value []byte) error

	// Delete removes a value.
	Delete(ctx context.Context, key []byte) error

	// Has checks if a key exists.
	Has(ctx context.Context, key []byte) (bool, error)

	// Iterate iterates over key-value pairs with a prefix.
	Iterate(ctx context.Context, prefix []byte, fn func(key, value []byte) error) error

	// Transaction executes a function within a transaction.
	Transaction(ctx context.Context, fn func(tx StorageTransaction) error) error
}

// StorageTransaction represents a storage transaction.
type StorageTransaction interface {
	// Get retrieves a value.
	Get(key []byte) ([]byte, error)

	// Set stores a value.
	Set(key, value []byte) error

	// Delete removes a value.
	Delete(key []byte) error
}

// VectorPlugin provides vector storage and similarity search capabilities.
type VectorPlugin interface {
	Plugin

	// Store stores a vector with metadata.
	Store(ctx context.Context, doc VectorDocument) error

	// StoreBatch stores multiple vectors.
	StoreBatch(ctx context.Context, docs []VectorDocument) error

	// Search finds similar vectors.
	Search(ctx context.Context, vector []float32, opts SearchOptions) ([]SearchResult, error)

	// Delete removes a vector by ID.
	Delete(ctx context.Context, id string) error

	// Get retrieves a vector by ID.
	Get(ctx context.Context, id string) (*VectorDocument, error)
}

// VectorDocument represents a document with its embedding.
type VectorDocument struct {
	// ID is the unique identifier.
	ID string `json:"id"`

	// Content is the original text content.
	Content string `json:"content"`

	// Embedding is the vector representation.
	Embedding []float32 `json:"embedding"`

	// Metadata contains additional metadata.
	Metadata map[string]any `json:"metadata,omitempty"`

	// Collection is the collection name.
	Collection string `json:"collection,omitempty"`
}

// SearchOptions configures a vector search.
type SearchOptions struct {
	// TopK is the number of results to return.
	TopK int `json:"topK"`

	// MinScore is the minimum similarity score (0-1).
	MinScore float32 `json:"minScore,omitempty"`

	// Collection limits search to a specific collection.
	Collection string `json:"collection,omitempty"`

	// Filter is a metadata filter.
	Filter map[string]any `json:"filter,omitempty"`
}

// SearchResult is a search result.
type SearchResult struct {
	// Document is the matching document.
	Document VectorDocument `json:"document"`

	// Score is the similarity score (0-1).
	Score float32 `json:"score"`
}

// NotificationPlugin provides notification delivery capabilities.
type NotificationPlugin interface {
	Plugin

	// Notify sends a notification.
	Notify(ctx context.Context, notification Notification) error

	// NotifyBatch sends multiple notifications.
	NotifyBatch(ctx context.Context, notifications []Notification) error
}

// Notification represents a notification to be sent.
type Notification struct {
	// Type is the notification type (e.g., "lock_conflict", "context_shared").
	Type string `json:"type"`

	// Title is the notification title.
	Title string `json:"title"`

	// Message is the notification body.
	Message string `json:"message"`

	// Data contains additional data.
	Data map[string]any `json:"data,omitempty"`

	// Severity indicates the importance (info, warning, error).
	Severity string `json:"severity,omitempty"`
}

// PluginRegistry manages plugin lifecycle.
type PluginRegistry struct {
	plugins map[string]Plugin
}

// NewPluginRegistry creates a new registry.
func NewPluginRegistry() *PluginRegistry {
	return &PluginRegistry{
		plugins: make(map[string]Plugin),
	}
}

// Register adds a plugin to the registry.
func (r *PluginRegistry) Register(p Plugin) {
	r.plugins[p.Name()] = p
}

// Get retrieves a plugin by name.
func (r *PluginRegistry) Get(name string) Plugin {
	return r.plugins[name]
}

// StartAll starts all registered plugins.
func (r *PluginRegistry) StartAll(ctx context.Context) error {
	for _, p := range r.plugins {
		if err := p.Start(ctx); err != nil {
			return err
		}
	}
	return nil
}

// StopAll stops all registered plugins.
func (r *PluginRegistry) StopAll() error {
	var lastErr error
	for _, p := range r.plugins {
		if err := p.Stop(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// All returns all registered plugins.
func (r *PluginRegistry) All() []Plugin {
	result := make([]Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		result = append(result, p)
	}
	return result
}
