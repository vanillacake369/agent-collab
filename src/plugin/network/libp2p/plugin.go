// Package libp2p provides a NetworkPlugin implementation using libp2p.
package libp2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	infraLibp2p "agent-collab/src/infrastructure/network/libp2p"
	"agent-collab/src/plugin"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Plugin wraps the libp2p Node as a NetworkPlugin.
type Plugin struct {
	node   *infraLibp2p.Node
	config *infraLibp2p.Config
	ctx    context.Context
	cancel context.CancelFunc

	subscriptions map[string]*subscriptionWrapper
	subMu         sync.RWMutex

	onConnectCallbacks    []func(string)
	onDisconnectCallbacks []func(string)
	callbackMu            sync.RWMutex

	ready   bool
	readyMu sync.RWMutex
}

type subscriptionWrapper struct {
	sub    *pubsub.Subscription
	cancel context.CancelFunc
}

// Config holds the plugin configuration.
type Config = infraLibp2p.Config

// NewPlugin creates a new libp2p network plugin.
func NewPlugin(cfg *Config) *Plugin {
	if cfg == nil {
		cfg = infraLibp2p.DefaultConfig()
	}
	return &Plugin{
		config:        cfg,
		subscriptions: make(map[string]*subscriptionWrapper),
	}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "network.libp2p"
}

// Start initializes and starts the libp2p node.
func (p *Plugin) Start(ctx context.Context) error {
	p.ctx, p.cancel = context.WithCancel(ctx)

	node, err := infraLibp2p.NewNode(p.ctx, p.config)
	if err != nil {
		return fmt.Errorf("failed to create libp2p node: %w", err)
	}
	p.node = node

	// Subscribe to project topics
	if err := node.SubscribeProjectTopics(p.ctx); err != nil {
		return fmt.Errorf("failed to subscribe project topics: %w", err)
	}

	// Bootstrap if peers provided
	if len(p.config.BootstrapPeers) > 0 {
		if err := node.Bootstrap(p.ctx, p.config.BootstrapPeers); err != nil {
			// Bootstrap failure is not fatal, just log it
			fmt.Printf("warning: bootstrap failed: %v\n", err)
		}
	}

	p.setReady(true)
	return nil
}

// Stop gracefully stops the libp2p node.
func (p *Plugin) Stop() error {
	p.setReady(false)

	// Cancel context first
	if p.cancel != nil {
		p.cancel()
	}

	// Close all subscriptions
	p.subMu.Lock()
	for _, sw := range p.subscriptions {
		sw.cancel()
		sw.sub.Cancel()
	}
	p.subscriptions = make(map[string]*subscriptionWrapper)
	p.subMu.Unlock()

	if p.node == nil {
		return nil
	}
	return p.node.Close()
}

// Ready returns true if the plugin is ready.
func (p *Plugin) Ready() bool {
	p.readyMu.RLock()
	defer p.readyMu.RUnlock()
	return p.ready
}

func (p *Plugin) setReady(ready bool) {
	p.readyMu.Lock()
	defer p.readyMu.Unlock()
	p.ready = ready
}

// ID returns the local peer ID.
func (p *Plugin) ID() string {
	if p.node == nil {
		return ""
	}
	return p.node.ID().String()
}

// Addresses returns the multiaddresses where this peer is listening.
func (p *Plugin) Addresses() []string {
	if p.node == nil {
		return nil
	}
	addrs := p.node.Addrs()
	result := make([]string, len(addrs))
	for i, addr := range addrs {
		result[i] = addr.String()
	}
	return result
}

// Peers returns information about connected peers.
func (p *Plugin) Peers() []plugin.PeerInfo {
	if p.node == nil {
		return nil
	}
	peerIDs := p.node.ConnectedPeers()
	result := make([]plugin.PeerInfo, len(peerIDs))
	for i, pid := range peerIDs {
		info := p.node.PeerInfo(pid)
		latency := p.node.Latency(pid)
		addrs := make([]string, len(info.Addrs))
		for j, addr := range info.Addrs {
			addrs[j] = addr.String()
		}
		result[i] = plugin.PeerInfo{
			ID:        pid.String(),
			Addresses: addrs,
			Latency:   latency,
			Connected: true,
		}
	}
	return result
}

// PeerCount returns the number of connected peers.
func (p *Plugin) PeerCount() int {
	if p.node == nil {
		return 0
	}
	return len(p.node.ConnectedPeers())
}

// Publish broadcasts a message to a topic.
func (p *Plugin) Publish(ctx context.Context, topic string, data []byte) error {
	if p.node == nil {
		return fmt.Errorf("node not started")
	}
	return p.node.Publish(ctx, topic, data)
}

// Subscribe subscribes to a topic and returns a channel for messages.
func (p *Plugin) Subscribe(ctx context.Context, topic string) (<-chan plugin.Message, error) {
	if p.node == nil {
		return nil, fmt.Errorf("node not started")
	}

	// Check if already subscribed
	p.subMu.RLock()
	if _, exists := p.subscriptions[topic]; exists {
		p.subMu.RUnlock()
		return nil, fmt.Errorf("already subscribed to topic: %s", topic)
	}
	p.subMu.RUnlock()

	// Subscribe to the topic
	sub, err := p.node.Subscribe(topic)
	if err != nil {
		return nil, err
	}

	// Create context for this subscription
	subCtx, subCancel := context.WithCancel(ctx)

	// Convert to plugin.Message channel
	msgCh := make(chan plugin.Message, 100)
	go func() {
		defer close(msgCh)
		for {
			msg, err := sub.Next(subCtx)
			if err != nil {
				// Context cancelled or subscription closed
				return
			}

			// Decompress and decrypt the message
			data, err := p.node.DecompressAndDecrypt(topic, msg.Data)
			if err != nil {
				// Fall back to raw data
				data = msg.Data
			}

			select {
			case msgCh <- plugin.Message{
				Topic:      topic,
				From:       msg.GetFrom().String(),
				Data:       data,
				ReceivedAt: time.Now(),
			}:
			case <-subCtx.Done():
				return
			}
		}
	}()

	p.subMu.Lock()
	p.subscriptions[topic] = &subscriptionWrapper{
		sub:    sub,
		cancel: subCancel,
	}
	p.subMu.Unlock()

	return msgCh, nil
}

// Unsubscribe unsubscribes from a topic.
func (p *Plugin) Unsubscribe(topic string) error {
	if p.node == nil {
		return fmt.Errorf("node not started")
	}

	p.subMu.Lock()
	sw, exists := p.subscriptions[topic]
	if !exists {
		p.subMu.Unlock()
		return fmt.Errorf("not subscribed to topic: %s", topic)
	}
	sw.cancel()
	sw.sub.Cancel()
	delete(p.subscriptions, topic)
	p.subMu.Unlock()

	return nil
}

// Connect connects to a specific peer.
func (p *Plugin) Connect(ctx context.Context, peerID string, addrs []string) error {
	if p.node == nil {
		return fmt.Errorf("node not started")
	}

	pid, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	// Get peer addresses from the node's host
	h := p.node.Host()
	addrInfo := h.Peerstore().PeerInfo(pid)
	if len(addrInfo.Addrs) == 0 {
		return fmt.Errorf("no addresses for peer: %s", peerID)
	}

	return h.Connect(ctx, addrInfo)
}

// Disconnect disconnects from a peer.
func (p *Plugin) Disconnect(peerID string) error {
	if p.node == nil {
		return fmt.Errorf("node not started")
	}

	pid, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}

	return p.node.Host().Network().ClosePeer(pid)
}

// OnPeerConnect registers a callback for when a peer connects.
func (p *Plugin) OnPeerConnect(callback func(peerID string)) {
	p.callbackMu.Lock()
	defer p.callbackMu.Unlock()
	p.onConnectCallbacks = append(p.onConnectCallbacks, callback)
}

// OnPeerDisconnect registers a callback for when a peer disconnects.
func (p *Plugin) OnPeerDisconnect(callback func(peerID string)) {
	p.callbackMu.Lock()
	defer p.callbackMu.Unlock()
	p.onDisconnectCallbacks = append(p.onDisconnectCallbacks, callback)
}

// Node returns the underlying libp2p Node for advanced use cases.
// This should be used sparingly as it breaks the abstraction.
func (p *Plugin) Node() *infraLibp2p.Node {
	return p.node
}

// Verify Plugin implements NetworkPlugin interface
var _ plugin.NetworkPlugin = (*Plugin)(nil)
