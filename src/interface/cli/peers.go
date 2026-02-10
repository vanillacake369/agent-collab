package cli

import (
	"fmt"
	"strings"

	"agent-collab/src/application"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/spf13/cobra"
)

var peersCmd = &cobra.Command{
	Use:   "peers",
	Short: "Peer 관리",
	Long:  `연결된 Peer를 관리합니다.`,
}

var peersListCmd = &cobra.Command{
	Use:   "list",
	Short: "Peer 목록",
	RunE:  runPeersList,
}

var peersInfoCmd = &cobra.Command{
	Use:   "info <peer-id>",
	Short: "Peer 상세 정보",
	Args:  cobra.ExactArgs(1),
	RunE:  runPeersInfo,
}

var peersBanCmd = &cobra.Command{
	Use:   "ban <peer-id>",
	Short: "Peer 차단",
	Args:  cobra.ExactArgs(1),
	RunE:  runPeersBan,
}

func init() {
	rootCmd.AddCommand(peersCmd)

	peersCmd.AddCommand(peersListCmd)
	peersCmd.AddCommand(peersInfoCmd)
	peersCmd.AddCommand(peersBanCmd)
}

func runPeersList(cmd *cobra.Command, args []string) error {
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	node := app.Node()
	if node == nil {
		fmt.Println("❌ 클러스터가 초기화되지 않았습니다.")
		fmt.Println("먼저 'agent-collab init' 또는 'agent-collab join'을 실행하세요.")
		return nil
	}

	peers := node.ConnectedPeers()

	fmt.Println("=== Connected Peers ===")
	fmt.Printf("Total: %d peers\n", len(peers))
	fmt.Println()

	if len(peers) == 0 {
		fmt.Println("연결된 peer가 없습니다.")
		return nil
	}

	fmt.Printf("%-8s %-50s %s\n", "STATUS", "PEER ID", "ADDRESSES")
	fmt.Println("─────────────────────────────────────────────────────────────────────────────────")

	for _, peerID := range peers {
		peerInfo := node.PeerInfo(peerID)
		addrStr := "-"
		if len(peerInfo.Addrs) > 0 {
			addrStr = peerInfo.Addrs[0].String()
			if len(addrStr) > 30 {
				addrStr = addrStr[:27] + "..."
			}
		}

		fmt.Printf("%-8s %-50s %s\n", "●", peerID.String(), addrStr)
	}

	return nil
}

func runPeersInfo(cmd *cobra.Command, args []string) error {
	peerIDStr := args[0]

	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	node := app.Node()
	if node == nil {
		return fmt.Errorf("클러스터가 초기화되지 않았습니다")
	}

	// Parse peer ID
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		return fmt.Errorf("잘못된 peer ID: %w", err)
	}

	peerInfo := node.PeerInfo(peerID)

	fmt.Printf("=== Peer Info: %s ===\n", peerID.String())
	fmt.Println()

	fmt.Printf("  %-14s: %s\n", "Peer ID", peerID.String())
	fmt.Printf("  %-14s: %s\n", "Status", "● Online")

	if len(peerInfo.Addrs) > 0 {
		fmt.Printf("  %-14s:\n", "Addresses")
		for _, addr := range peerInfo.Addrs {
			fmt.Printf("    - %s\n", addr.String())
		}
	} else {
		fmt.Printf("  %-14s: (no addresses)\n", "Addresses")
	}

	// Extract transport from first address
	if len(peerInfo.Addrs) > 0 {
		addrStr := peerInfo.Addrs[0].String()
		transport := "TCP"
		if strings.Contains(addrStr, "quic") {
			transport = "QUIC"
		} else if strings.Contains(addrStr, "webrtc") {
			transport = "WebRTC"
		} else if strings.Contains(addrStr, "ws") {
			transport = "WebSocket"
		}
		fmt.Printf("  %-14s: %s\n", "Transport", transport)
	}

	return nil
}

func runPeersBan(cmd *cobra.Command, args []string) error {
	peerIDStr := args[0]

	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	node := app.Node()
	if node == nil {
		return fmt.Errorf("클러스터가 초기화되지 않았습니다")
	}

	// Parse peer ID
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		return fmt.Errorf("잘못된 peer ID: %w", err)
	}

	fmt.Printf("⚠️  Peer 차단: %s\n", peerID.String())
	fmt.Println()

	// Note: libp2p doesn't have built-in ban functionality
	// This would need to be implemented with connection gating
	fmt.Println("⚠️  Peer 차단 기능은 아직 구현되지 않았습니다.")
	fmt.Println("  향후 버전에서 지원될 예정입니다.")

	return nil
}
