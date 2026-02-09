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
	ps, err := pubsub.NewGossipSub(ctx, h,
		pubsub.WithPeerExchange(true),
		pubsub.WithFloodPublish(true),
	)
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
func (n *Node) Subscribe(topicName string) (*pubsub.Subscription, error) {
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
func (n *Node) Publish(ctx context.Context, topicName string, data []byte) error {
	topic, err := n.JoinTopic(topicName)
	if err != nil {
		return err
	}

	return topic.Publish(ctx, data)
}

// ConnectedPeers는 연결된 peer 목록을 반환합니다.
func (n *Node) ConnectedPeers() []peer.ID {
	return n.host.Network().Peers()
}

// PeerInfo는 peer 정보를 반환합니다.
func (n *Node) PeerInfo(id peer.ID) peer.AddrInfo {
	return n.host.Peerstore().PeerInfo(id)
}

// Latency returns the latency to a peer.
func (n *Node) Latency(id peer.ID) time.Duration {
	return n.host.Peerstore().LatencyEWMA(id)
}

// Close는 노드를 종료합니다.
func (n *Node) Close() error {
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
