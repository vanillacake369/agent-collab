package libp2p

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

// DiscoveryService는 peer 발견 서비스입니다.
type DiscoveryService struct {
	host      host.Host
	projectID string

	// mDNS 서비스
	mdnsService mdns.Service

	// 발견된 peer 콜백
	onPeerFound func(peer.AddrInfo)

	mu sync.RWMutex
}

// DiscoveryNotifee는 mDNS 알림 수신자입니다.
type DiscoveryNotifee struct {
	h           host.Host
	onPeerFound func(peer.AddrInfo)
}

// HandlePeerFound는 peer 발견 시 호출됩니다.
func (n *DiscoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	// 자기 자신은 무시
	if pi.ID == n.h.ID() {
		return
	}

	// 연결 시도
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := n.h.Connect(ctx, pi); err != nil {
		fmt.Printf("peer 연결 실패 (%s): %v\n", pi.ID.String()[:8], err)
		return
	}

	fmt.Printf("peer 발견 및 연결: %s\n", pi.ID.String()[:8])

	if n.onPeerFound != nil {
		n.onPeerFound(pi)
	}
}

// NewDiscoveryService는 새 발견 서비스를 생성합니다.
func NewDiscoveryService(h host.Host, projectID string) *DiscoveryService {
	return &DiscoveryService{
		host:      h,
		projectID: projectID,
	}
}

// SetPeerFoundHandler는 peer 발견 핸들러를 설정합니다.
func (d *DiscoveryService) SetPeerFoundHandler(handler func(peer.AddrInfo)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onPeerFound = handler
}

// StartMDNS는 mDNS 서비스를 시작합니다.
func (d *DiscoveryService) StartMDNS(ctx context.Context) error {
	// mDNS 서비스 이름 (프로젝트별)
	serviceName := "agent-collab-" + d.projectID

	notifee := &DiscoveryNotifee{
		h:           d.host,
		onPeerFound: d.onPeerFound,
	}

	service := mdns.NewMdnsService(d.host, serviceName, notifee)

	d.mu.Lock()
	d.mdnsService = service
	d.mu.Unlock()

	return service.Start()
}

// Close는 발견 서비스를 종료합니다.
func (d *DiscoveryService) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.mdnsService != nil {
		return d.mdnsService.Close()
	}

	return nil
}

// BootstrapPeers는 공용 bootstrap peer 목록을 반환합니다.
func BootstrapPeers() []peer.AddrInfo {
	// IPFS 공용 bootstrap 노드
	bootstrapAddrs := []string{
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
	}

	var peers []peer.AddrInfo
	for _, addr := range bootstrapAddrs {
		pi, err := peer.AddrInfoFromString(addr)
		if err != nil {
			continue
		}
		peers = append(peers, *pi)
	}

	return peers
}
