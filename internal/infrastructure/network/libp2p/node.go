package libp2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/net/connmgr"
	"github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/multiformats/go-multiaddr"
)

// Node는 libp2p 노드입니다.
type Node struct {
	host      host.Host
	dht       *dht.IpfsDHT
	pubsub    *pubsub.PubSub
	topics    map[string]*pubsub.Topic
	subs      map[string]*pubsub.Subscription
	projectID string

	// Phase 1: Compression, Batching, Metrics
	batcher *MessageBatcher
	metrics *NetworkMetrics

	// Phase 2: Quality, Topology, Locality, ACL
	qualityMonitor *PeerQualityMonitor
	topologyMgr    *TopologyManager
	localityMgr    *LocalityManager
	aclMgr         *ACLManager

	// Phase 3: Tracing
	tracer *Tracer

	// Phase 2: Content Store
	contentStore *ContentStore

	mu sync.RWMutex
}

// Config는 노드 설정입니다.
type Config struct {
	// 리스닝 주소
	ListenAddrs []string

	// Bootstrap peer 주소
	BootstrapPeers []peer.AddrInfo

	// 프로젝트 ID (Rendezvous namespace)
	ProjectID string

	// 개인 키 (nil이면 새로 생성)
	PrivateKey crypto.PrivKey

	// 연결 관리 설정
	LowWater  int
	HighWater int

	// Phase 1: 메시지 배칭 설정 (nil이면 배칭 비활성화)
	BatchConfig *BatchConfig

	// Phase 1: GossipSub 설정 (nil이면 기본 설정 사용)
	GossipConfig *GossipSubConfig

	// Phase 2: 피어 품질 모니터링 (nil이면 비활성화)
	QualityConfig *PeerQualityConfig

	// Phase 2: 계층적 토폴로지 (nil이면 비활성화)
	TopologyConfig *TopologyConfig

	// Phase 2: 지역성 클러스터링 (nil이면 비활성화)
	LocalityConfig *LocalityConfig

	// Phase 2: ACL 정책 (기본: PolicyAllowAll)
	ACLPolicy ACLPolicy

	// Phase 2: 컨텐츠 스토어 (nil이면 기본 설정)
	ContentStoreConfig *ContentStoreConfig

	// Phase 3: 분산 트레이싱 (nil이면 비활성화)
	TracerConfig *TracerConfig
}

// DefaultConfig는 기본 설정을 반환합니다.
func DefaultConfig() *Config {
	return &Config{
		ListenAddrs: []string{
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
		},
		LowWater:  100,
		HighWater: 400,
	}
}

// NewNode는 새 libp2p 노드를 생성합니다.
func NewNode(ctx context.Context, cfg *Config) (*Node, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	// 개인 키 생성 (없으면)
	privKey := cfg.PrivateKey
	if privKey == nil {
		var err error
		privKey, _, err = crypto.GenerateKeyPair(crypto.Ed25519, -1)
		if err != nil {
			return nil, fmt.Errorf("키 생성 실패: %w", err)
		}
	}

	// 연결 관리자 생성
	connMgr, err := connmgr.NewConnManager(
		cfg.LowWater,
		cfg.HighWater,
		connmgr.WithGracePeriod(time.Minute),
	)
	if err != nil {
		return nil, fmt.Errorf("연결 관리자 생성 실패: %w", err)
	}

	// 리스닝 주소 변환
	var listenAddrs []multiaddr.Multiaddr
	for _, addr := range cfg.ListenAddrs {
		ma, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("주소 파싱 실패 (%s): %w", addr, err)
		}
		listenAddrs = append(listenAddrs, ma)
	}

	// libp2p 호스트 생성
	h, err := libp2p.New(
		libp2p.Identity(privKey),
		libp2p.ListenAddrs(listenAddrs...),

		// 보안
		libp2p.Security(noise.ID, noise.New),

		// NAT 통과
		libp2p.NATPortMap(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableHolePunching(),
		libp2p.EnableRelayService(),

		// 연결 관리
		libp2p.ConnectionManager(connMgr),
	)
	if err != nil {
		return nil, fmt.Errorf("호스트 생성 실패: %w", err)
	}

	// DHT 초기화
	kadDHT, err := dht.New(ctx, h,
		dht.Mode(dht.ModeAutoServer),
	)
	if err != nil {
		h.Close()
		return nil, fmt.Errorf("DHT 생성 실패: %w", err)
	}

	// Gossipsub 초기화
	gossipOpts := []pubsub.Option{
		pubsub.WithPeerExchange(true),
		pubsub.WithFloodPublish(true),
	}

	// Apply custom GossipSub config if provided
	if cfg.GossipConfig != nil {
		gossipOpts = cfg.GossipConfig.ToOptions()
	}

	ps, err := pubsub.NewGossipSub(ctx, h, gossipOpts...)
	if err != nil {
		kadDHT.Close()
		h.Close()
		return nil, fmt.Errorf("PubSub 생성 실패: %w", err)
	}

	node := &Node{
		host:      h,
		dht:       kadDHT,
		pubsub:    ps,
		topics:    make(map[string]*pubsub.Topic),
		subs:      make(map[string]*pubsub.Subscription),
		projectID: cfg.ProjectID,
		metrics:   NewNetworkMetrics(),
	}

	// Phase 1: Initialize batcher if configured
	if cfg.BatchConfig != nil {
		node.batcher = NewMessageBatcher(*cfg.BatchConfig, node.publishDirect)
		node.batcher.Start(ctx)
	}

	// Phase 2: Initialize peer quality monitor
	if cfg.QualityConfig != nil {
		node.qualityMonitor = NewPeerQualityMonitor(h, *cfg.QualityConfig)
		node.qualityMonitor.Start()
	} else {
		// Default quality monitor for topology/locality
		node.qualityMonitor = NewPeerQualityMonitor(h, DefaultPeerQualityConfig())
		node.qualityMonitor.Start()
	}

	// Phase 2: Initialize topology manager
	if cfg.TopologyConfig != nil {
		criteria := DefaultSuperPeerCriteria()
		node.topologyMgr = NewTopologyManager(h, *cfg.TopologyConfig, criteria)
		node.topologyMgr.SetQualityMonitor(node.qualityMonitor)
		node.topologyMgr.Start()
	}

	// Phase 2: Initialize locality manager
	if cfg.LocalityConfig != nil {
		node.localityMgr = NewLocalityManager(h, *cfg.LocalityConfig)
		node.localityMgr.SetQualityMonitor(node.qualityMonitor)
		node.localityMgr.Start()
	}

	// Phase 2: Initialize ACL manager
	node.aclMgr = NewACLManager(cfg.ACLPolicy)

	// Phase 2: Initialize content store
	if cfg.ContentStoreConfig != nil {
		node.contentStore = NewContentStore(*cfg.ContentStoreConfig)
	} else {
		node.contentStore = NewContentStore(DefaultContentStoreConfig())
	}

	// Phase 3: Initialize tracer
	if cfg.TracerConfig != nil {
		node.tracer = NewTracer(*cfg.TracerConfig)
	}

	return node, nil
}

// Bootstrap은 DHT bootstrap을 수행합니다.
func (n *Node) Bootstrap(ctx context.Context, peers []peer.AddrInfo) error {
	// Bootstrap peer에 연결
	var wg sync.WaitGroup
	for _, peerInfo := range peers {
		wg.Add(1)
		go func(pi peer.AddrInfo) {
			defer wg.Done()
			if err := n.host.Connect(ctx, pi); err != nil {
				fmt.Printf("Bootstrap peer 연결 실패 (%s): %v\n", pi.ID, err)
			}
		}(peerInfo)
	}
	wg.Wait()

	// DHT bootstrap
	return n.dht.Bootstrap(ctx)
}

// ID는 노드 ID를 반환합니다.
func (n *Node) ID() peer.ID {
	return n.host.ID()
}

// Addrs는 노드 주소를 반환합니다.
func (n *Node) Addrs() []multiaddr.Multiaddr {
	return n.host.Addrs()
}

// Host는 libp2p 호스트를 반환합니다.
func (n *Node) Host() host.Host {
	return n.host
}

// JoinTopic은 토픽에 참여합니다.
func (n *Node) JoinTopic(topicName string) (*pubsub.Topic, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	if topic, exists := n.topics[topicName]; exists {
		return topic, nil
	}

	topic, err := n.pubsub.Join(topicName)
	if err != nil {
		return nil, err
	}

	n.topics[topicName] = topic
	return topic, nil
}

// Subscribe는 토픽을 구독합니다.
// ACL 체크를 수행하여 권한이 없으면 거부합니다.
func (n *Node) Subscribe(topicName string) (*pubsub.Subscription, error) {
	// Phase 2: ACL check
	if n.aclMgr != nil && !n.aclMgr.CanSubscribe(topicName, n.host.ID()) {
		n.metrics.RecordError("subscribe_acl_denied")
		return nil, ErrAccessDenied
	}

	topic, err := n.JoinTopic(topicName)
	if err != nil {
		return nil, err
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	if sub, exists := n.subs[topicName]; exists {
		return sub, nil
	}

	sub, err := topic.Subscribe()
	if err != nil {
		return nil, err
	}

	n.subs[topicName] = sub
	return sub, nil
}

// Publish는 토픽에 메시지를 발행합니다.
// ACL 체크, 암호화, 배칭, 압축을 순차적으로 적용합니다.
func (n *Node) Publish(ctx context.Context, topicName string, data []byte) error {
	// Phase 3: Start tracing span
	var span *Span
	if n.tracer != nil {
		span, ctx = n.tracer.StartSpanFromContext(ctx, "publish")
		if span != nil {
			span.SetTag("topic", topicName)
			span.SetTag("size", fmt.Sprintf("%d", len(data)))
			defer n.tracer.EndSpan(span)
		}
	}

	// Phase 2: ACL check
	if n.aclMgr != nil && !n.aclMgr.CanPublish(topicName, n.host.ID()) {
		if span != nil {
			span.SetError()
			span.SetTag("error", "access_denied")
		}
		n.metrics.RecordError("publish_acl_denied")
		return ErrAccessDenied
	}

	// Phase 2: Encrypt if ACL has encryption enabled
	var err error
	if n.aclMgr != nil {
		data, err = n.aclMgr.Encrypt(topicName, data)
		if err != nil {
			if span != nil {
				span.SetError()
			}
			n.metrics.RecordError("publish_encrypt_failed")
			return err
		}
	}

	// If batching is enabled, add to batch
	if n.batcher != nil {
		return n.batcher.Add(ctx, topicName, data)
	}

	// Otherwise publish directly with compression
	return n.publishDirect(ctx, topicName, data)
}

// publishDirect publishes a message directly (used by batcher)
func (n *Node) publishDirect(ctx context.Context, topicName string, data []byte) error {
	topic, err := n.JoinTopic(topicName)
	if err != nil {
		n.metrics.RecordError("publish_join_topic")
		return err
	}

	// Apply compression
	originalSize := len(data)
	compressed := CompressMessage(data)
	compressedSize := len(compressed)

	// Record metrics
	n.metrics.RecordCompression(originalSize, compressedSize)
	n.metrics.RecordMessageSent(topicName, "message", compressedSize)

	err = topic.Publish(ctx, compressed)
	if err != nil {
		n.metrics.RecordError("publish_failed")
	}
	return err
}

// PublishRaw는 압축/배칭 없이 메시지를 발행합니다.
func (n *Node) PublishRaw(ctx context.Context, topicName string, data []byte) error {
	topic, err := n.JoinTopic(topicName)
	if err != nil {
		return err
	}

	return topic.Publish(ctx, data)
}

// FlushBatch flushes pending batched messages for a topic
func (n *Node) FlushBatch(ctx context.Context, topicName string) error {
	if n.batcher == nil {
		return nil
	}
	return n.batcher.Flush(ctx, topicName)
}

// FlushAllBatches flushes all pending batched messages
func (n *Node) FlushAllBatches(ctx context.Context) {
	if n.batcher != nil {
		n.batcher.FlushAll(ctx)
	}
}

// ConnectedPeers는 연결된 peer 목록을 반환합니다.
func (n *Node) ConnectedPeers() []peer.ID {
	return n.host.Network().Peers()
}

// PeerInfo는 peer 정보를 반환합니다.
func (n *Node) PeerInfo(id peer.ID) peer.AddrInfo {
	return n.host.Peerstore().PeerInfo(id)
}

// Latency는 peer의 지연 시간을 반환합니다.
func (n *Node) Latency(id peer.ID) time.Duration {
	return n.host.Peerstore().LatencyEWMA(id)
}

// Close는 노드를 종료합니다.
func (n *Node) Close() error {
	// Phase 1: Stop batcher first to flush pending messages
	if n.batcher != nil {
		n.batcher.Stop()
	}

	// Phase 3: Flush tracer spans
	if n.tracer != nil {
		_ = n.tracer.Flush(context.Background())
	}

	// Phase 2: Stop managers
	if n.topologyMgr != nil {
		n.topologyMgr.Stop()
	}
	if n.localityMgr != nil {
		n.localityMgr.Stop()
	}
	if n.qualityMonitor != nil {
		n.qualityMonitor.Stop()
	}

	n.mu.Lock()
	defer n.mu.Unlock()

	// 구독 취소
	for _, sub := range n.subs {
		sub.Cancel()
	}

	// 토픽 닫기
	for _, topic := range n.topics {
		topic.Close()
	}

	// DHT 닫기
	if err := n.dht.Close(); err != nil {
		return err
	}

	// 호스트 닫기
	return n.host.Close()
}

// ProjectTopics는 프로젝트 토픽 이름 목록을 반환합니다.
func (n *Node) ProjectTopics() []string {
	prefix := "/agent-collab/" + n.projectID
	return []string{
		prefix + "/context",
		prefix + "/lock",
		prefix + "/vibe",
		prefix + "/human",
	}
}

// SubscribeProjectTopics는 프로젝트 토픽들을 구독합니다.
func (n *Node) SubscribeProjectTopics(ctx context.Context) error {
	for _, topicName := range n.ProjectTopics() {
		if _, err := n.Subscribe(topicName); err != nil {
			return fmt.Errorf("토픽 구독 실패 (%s): %w", topicName, err)
		}
	}
	return nil
}

// GetSubscription returns the subscription for a topic.
func (n *Node) GetSubscription(topicName string) *pubsub.Subscription {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.subs[topicName]
}

// Metrics returns the network metrics collector
func (n *Node) Metrics() *NetworkMetrics {
	return n.metrics
}

// GetMetricsSnapshot returns a snapshot of current metrics
func (n *Node) GetMetricsSnapshot() MetricsSnapshot {
	// Update peer count
	n.metrics.mu.Lock()
	n.metrics.peersConnected = len(n.host.Network().Peers())
	n.metrics.mu.Unlock()

	return n.metrics.Snapshot()
}

// Phase 2-3 Manager Accessors

// QualityMonitor returns the peer quality monitor
func (n *Node) QualityMonitor() *PeerQualityMonitor {
	return n.qualityMonitor
}

// TopologyManager returns the topology manager
func (n *Node) TopologyManager() *TopologyManager {
	return n.topologyMgr
}

// LocalityManager returns the locality manager
func (n *Node) LocalityManager() *LocalityManager {
	return n.localityMgr
}

// ACLManager returns the ACL manager
func (n *Node) ACLManager() *ACLManager {
	return n.aclMgr
}

// ContentStore returns the content store
func (n *Node) ContentStore() *ContentStore {
	return n.contentStore
}

// Tracer returns the distributed tracer
func (n *Node) Tracer() *Tracer {
	return n.tracer
}

// DecryptMessage decrypts a received message using the topic's ACL
func (n *Node) DecryptMessage(topicName string, data []byte) ([]byte, error) {
	if n.aclMgr == nil {
		return data, nil
	}
	return n.aclMgr.Decrypt(topicName, data)
}

// DecompressAndDecrypt decompresses and decrypts a received message
func (n *Node) DecompressAndDecrypt(topicName string, data []byte) ([]byte, error) {
	// First decompress
	decompressed, err := DecompressMessage(data)
	if err != nil {
		return nil, err
	}

	// Then decrypt
	return n.DecryptMessage(topicName, decompressed)
}
