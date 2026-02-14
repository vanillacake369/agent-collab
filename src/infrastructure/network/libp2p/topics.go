package libp2p

// Global topic constants for P2P communication.
// These topics are cluster-wide (no projectID scoping).
const (
	// Cluster membership topics
	TopicClusterJoin  = "/agent-collab/cluster/join"
	TopicClusterLeave = "/agent-collab/cluster/leave"
	TopicClusterPing  = "/agent-collab/cluster/ping"

	// Event topics (interest-based filtering is done at receive side)
	TopicEvents = "/agent-collab/events"

	// Lock topics (global, no filtering)
	TopicLockIntent  = "/agent-collab/locks/intent"
	TopicLockAcquire = "/agent-collab/locks/acquire"
	TopicLockRelease = "/agent-collab/locks/release"

	// Context synchronization
	TopicContextSync = "/agent-collab/context/sync"

	// Interest synchronization (share interests across cluster)
	TopicInterestSync = "/agent-collab/interest/sync"
)

// AllGlobalTopics returns all global topics for subscription.
func AllGlobalTopics() []string {
	return []string{
		TopicClusterJoin,
		TopicClusterLeave,
		TopicClusterPing,
		TopicEvents,
		TopicLockIntent,
		TopicLockAcquire,
		TopicLockRelease,
		TopicContextSync,
		TopicInterestSync,
	}
}

// CoreTopics returns the minimum set of topics for basic operation.
func CoreTopics() []string {
	return []string{
		TopicEvents,
		TopicLockIntent,
		TopicLockAcquire,
		TopicLockRelease,
		TopicContextSync,
	}
}

// ClusterTopics returns topics for cluster membership management.
func ClusterTopics() []string {
	return []string{
		TopicClusterJoin,
		TopicClusterLeave,
		TopicClusterPing,
	}
}
