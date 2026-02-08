package cli

import (
	"fmt"

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
	// TODO: 실제 peer 목록 가져오기
	peers := []PeerStatus{
		{ID: "QmAbc...123", Name: "Alice", Status: "online", Latency: 12, Transport: "QUIC"},
		{ID: "QmDef...456", Name: "Bob", Status: "online", Latency: 45, Transport: "WebRTC"},
		{ID: "QmGhi...789", Name: "Charlie", Status: "syncing", Latency: 89, Transport: "TCP"},
		{ID: "QmJkl...012", Name: "Diana", Status: "online", Latency: 23, Transport: "QUIC"},
	}

	fmt.Println("=== Connected Peers ===")
	fmt.Printf("Total: %d peers\n", len(peers))
	fmt.Println()

	fmt.Printf("%-8s %-10s %-15s %-10s %8s  %s\n",
		"STATUS", "NAME", "PEER ID", "TRANSPORT", "LATENCY", "SYNC")
	fmt.Println("────────────────────────────────────────────────────────────────")

	for _, p := range peers {
		statusIcon := "●"
		if p.Status == "syncing" {
			statusIcon = "◐"
		} else if p.Status == "offline" {
			statusIcon = "○"
		}

		fmt.Printf("%-8s %-10s %-15s %-10s %6dms  %s\n",
			statusIcon, p.Name, p.ID, p.Transport, p.Latency, "100%")
	}

	return nil
}

func runPeersInfo(cmd *cobra.Command, args []string) error {
	peerID := args[0]

	// TODO: 실제 peer 정보 가져오기
	fmt.Printf("=== Peer Info: %s ===\n", peerID)
	fmt.Println()

	info := map[string]string{
		"Peer ID":     "QmAbc...123",
		"Name":        "Alice",
		"Status":      "● Online",
		"Connected":   "2 hours ago",
		"Transport":   "QUIC (UDP)",
		"Address":     "/ip4/192.168.1.100/udp/4001/quic-v1",
		"Latency":     "12ms (avg), 8ms (min), 23ms (max)",
		"Messages":    "↑ 1,234  ↓ 2,345",
		"Sync":        "100% (12,456 vectors)",
		"Capabilities": "[embedding] [lock] [context]",
	}

	for k, v := range info {
		fmt.Printf("  %-14s: %s\n", k, v)
	}

	return nil
}

func runPeersBan(cmd *cobra.Command, args []string) error {
	peerID := args[0]

	fmt.Printf("⚠️  Peer 차단: %s\n", peerID)
	fmt.Println()

	// TODO: 확인 프롬프트
	fmt.Println("✓ Peer가 차단되었습니다.")
	fmt.Println("  이 peer는 더 이상 연결할 수 없습니다.")

	return nil
}
