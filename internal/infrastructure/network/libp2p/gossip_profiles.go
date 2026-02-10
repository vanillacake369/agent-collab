package libp2p

import (
	"time"

	pubsub "github.com/libp2p/go-libp2p-pubsub"
)

// GossipProfile defines the cluster size profile for GossipSub tuning
type GossipProfile string

const (
	// ProfileSmall is for 2-10 peers - minimal overhead
	ProfileSmall GossipProfile = "small"
	// ProfileMedium is for 10-50 peers - balanced settings
	ProfileMedium GossipProfile = "medium"
	// ProfileLarge is for 50-200 peers - optimized for scale
	ProfileLarge GossipProfile = "large"
	// ProfileXLarge is for 200+ peers - maximum scale optimization
	ProfileXLarge GossipProfile = "xlarge"
)

// GossipParams contains tunable GossipSub parameters
type GossipParams struct {
	// D is the desired outbound degree of the network (mesh size)
	D int
	// Dlo is the minimum outbound degree before we try to expand
	Dlo int
	// Dhi is the maximum outbound degree before we start pruning
	Dhi int
	// Dlazy is the degree for gossip emission
	Dlazy int
	// HeartbeatInterval is the time between heartbeats
	HeartbeatInterval time.Duration
	// HistoryGossip is the number of heartbeat intervals to keep message IDs for gossip
	HistoryGossip int
	// HistoryLength is the number of heartbeat intervals to keep message history
	HistoryLength int
	// FanoutTTL is how long to keep fanout state for topics we publish to
	FanoutTTL time.Duration
	// MaxMessageSize is the maximum message size (0 = default 1MB)
	MaxMessageSize int
}

// DefaultGossipParams returns default GossipSub parameters (medium profile)
func DefaultGossipParams() GossipParams {
	return ProfileParams[ProfileMedium]
}

// ProfileParams maps profiles to their optimal parameters
var ProfileParams = map[GossipProfile]GossipParams{
	ProfileSmall: {
		D:                 4, // Smaller mesh for few peers
		Dlo:               2, // Lower bound
		Dhi:               6, // Upper bound
		Dlazy:             4, // Lazy push degree
		HeartbeatInterval: 700 * time.Millisecond,
		HistoryGossip:     3,
		HistoryLength:     5,
		FanoutTTL:         30 * time.Second,
		MaxMessageSize:    1 << 20, // 1MB
	},
	ProfileMedium: {
		D:                 6,
		Dlo:               4,
		Dhi:               8,
		Dlazy:             6,
		HeartbeatInterval: 1 * time.Second,
		HistoryGossip:     4,
		HistoryLength:     6,
		FanoutTTL:         45 * time.Second,
		MaxMessageSize:    1 << 20, // 1MB
	},
	ProfileLarge: {
		D:                 8,
		Dlo:               6,
		Dhi:               12,
		Dlazy:             8,
		HeartbeatInterval: 1 * time.Second,
		HistoryGossip:     5,
		HistoryLength:     8,
		FanoutTTL:         60 * time.Second,
		MaxMessageSize:    2 << 20, // 2MB for larger clusters
	},
	ProfileXLarge: {
		D:                 10,
		Dlo:               8,
		Dhi:               14,
		Dlazy:             10,
		HeartbeatInterval: 1200 * time.Millisecond,
		HistoryGossip:     6,
		HistoryLength:     10,
		FanoutTTL:         90 * time.Second,
		MaxMessageSize:    2 << 20, // 2MB
	},
}

// GetProfileForPeerCount returns the appropriate profile for a given peer count
func GetProfileForPeerCount(count int) GossipProfile {
	switch {
	case count <= 10:
		return ProfileSmall
	case count <= 50:
		return ProfileMedium
	case count <= 200:
		return ProfileLarge
	default:
		return ProfileXLarge
	}
}

// ToGossipSubOptions converts GossipParams to pubsub options
func (p GossipParams) ToGossipSubOptions() []pubsub.Option {
	opts := []pubsub.Option{
		pubsub.WithPeerExchange(true),
		pubsub.WithFloodPublish(true),
	}

	// Apply GossipSub-specific parameters if the library supports them
	// Note: Some parameters require specific pubsub versions
	if p.MaxMessageSize > 0 {
		opts = append(opts, pubsub.WithMaxMessageSize(p.MaxMessageSize))
	}

	return opts
}

// GossipSubConfig holds the complete GossipSub configuration
type GossipSubConfig struct {
	// Profile is the base profile to use
	Profile GossipProfile
	// Params are the actual parameters (can be customized from profile defaults)
	Params GossipParams
	// EnablePeerExchange enables peer exchange
	EnablePeerExchange bool
	// EnableFloodPublish enables flood publishing
	EnableFloodPublish bool
	// DirectPeers are peers to always connect to
	DirectPeers []string
}

// DefaultGossipSubConfig returns a default GossipSub configuration
func DefaultGossipSubConfig() GossipSubConfig {
	return GossipSubConfig{
		Profile:            ProfileMedium,
		Params:             ProfileParams[ProfileMedium],
		EnablePeerExchange: true,
		EnableFloodPublish: true,
	}
}

// WithProfile sets the profile and updates params accordingly
func (c GossipSubConfig) WithProfile(profile GossipProfile) GossipSubConfig {
	c.Profile = profile
	c.Params = ProfileParams[profile]
	return c
}

// ToOptions converts the config to pubsub options
func (c GossipSubConfig) ToOptions() []pubsub.Option {
	opts := []pubsub.Option{}

	if c.EnablePeerExchange {
		opts = append(opts, pubsub.WithPeerExchange(true))
	}
	if c.EnableFloodPublish {
		opts = append(opts, pubsub.WithFloodPublish(true))
	}
	if c.Params.MaxMessageSize > 0 {
		opts = append(opts, pubsub.WithMaxMessageSize(c.Params.MaxMessageSize))
	}

	return opts
}
