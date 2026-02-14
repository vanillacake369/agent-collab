package application

// InitResult는 초기화 결과입니다.
type InitResult struct {
	ProjectName string   `json:"project_name"`
	NodeID      string   `json:"node_id"`
	Addresses   []string `json:"addresses"`
	InviteToken string   `json:"invite_token"`
	KeyPath     string   `json:"key_path"`

	// WireGuard VPN info (optional)
	WireGuardEnabled  bool   `json:"wireguard_enabled,omitempty"`
	WireGuardIP       string `json:"wireguard_ip,omitempty"`
	WireGuardEndpoint string `json:"wireguard_endpoint,omitempty"`
}

// JoinResult는 참여 결과입니다.
type JoinResult struct {
	ProjectName    string `json:"project_name"`
	NodeID         string `json:"node_id"`
	BootstrapPeer  string `json:"bootstrap_peer"`
	ConnectedPeers int    `json:"connected_peers"`

	// WireGuard VPN info (optional)
	WireGuardEnabled bool   `json:"wireguard_enabled,omitempty"`
	WireGuardIP      string `json:"wireguard_ip,omitempty"`
}

// Status holds the application status.
type Status struct {
	Running      bool     `json:"running"`
	ProjectName  string   `json:"project_name"`
	NodeID       string   `json:"node_id"`
	Addresses    []string `json:"addresses"`
	PeerCount    int      `json:"peer_count"`
	LockCount    int      `json:"lock_count"`
	MyLockCount  int      `json:"my_lock_count"`
	DeltaCount   int      `json:"delta_count"`
	WatchedFiles int      `json:"watched_files"`

	// Token usage (Phase 3)
	TokensToday   int64   `json:"tokens_today"`
	TokensPerHour float64 `json:"tokens_per_hour"`
	CostToday     float64 `json:"cost_today"`

	// Vector store (Phase 3)
	EmbeddingCount int64 `json:"embedding_count"`

	// WireGuard VPN status
	WireGuardEnabled   bool   `json:"wireguard_enabled,omitempty"`
	WireGuardIP        string `json:"wireguard_ip,omitempty"`
	WireGuardEndpoint  string `json:"wireguard_endpoint,omitempty"`
	WireGuardPeerCount int    `json:"wireguard_peer_count,omitempty"`
}
