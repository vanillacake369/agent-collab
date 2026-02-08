package cli

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "클러스터 상태 확인",
	Long:  `현재 클러스터의 상태를 확인합니다.`,
	RunE:  runStatus,
}

var (
	statusJSON  bool
	statusWatch bool
)

func init() {
	rootCmd.AddCommand(statusCmd)

	statusCmd.Flags().BoolVar(&statusJSON, "json", false, "JSON 형식으로 출력")
	statusCmd.Flags().BoolVarP(&statusWatch, "watch", "w", false, "실시간 갱신")
}

// ClusterStatus는 클러스터 상태 정보입니다.
type ClusterStatus struct {
	ProjectName string       `json:"project_name"`
	NodeID      string       `json:"node_id"`
	Status      string       `json:"status"`
	Uptime      string       `json:"uptime"`
	Peers       []PeerStatus `json:"peers"`
	SyncHealth  float64      `json:"sync_health"`
	ActiveLocks int          `json:"active_locks"`
}

// PeerStatus는 peer 상태 정보입니다.
type PeerStatus struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Latency   int    `json:"latency_ms"`
	Transport string `json:"transport"`
}

func runStatus(cmd *cobra.Command, args []string) error {
	// TODO: 실제 상태 가져오기
	status := &ClusterStatus{
		ProjectName: "my-project",
		NodeID:      "QmXx...Yy",
		Status:      "connected",
		Uptime:      "2h 34m",
		SyncHealth:  98.5,
		ActiveLocks: 2,
		Peers: []PeerStatus{
			{ID: "QmAbc...123", Name: "Alice", Status: "online", Latency: 12, Transport: "QUIC"},
			{ID: "QmDef...456", Name: "Bob", Status: "online", Latency: 45, Transport: "WebRTC"},
			{ID: "QmGhi...789", Name: "Charlie", Status: "syncing", Latency: 89, Transport: "TCP"},
			{ID: "QmJkl...012", Name: "Diana", Status: "online", Latency: 23, Transport: "QUIC"},
		},
	}

	if statusWatch {
		return runStatusWatch(status)
	}

	return printStatus(status)
}

func printStatus(status *ClusterStatus) error {
	if statusJSON {
		data, err := json.MarshalIndent(status, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	// 일반 출력
	fmt.Println("=== Cluster Status ===")
	fmt.Printf("Project    : %s\n", status.ProjectName)
	fmt.Printf("Node ID    : %s\n", status.NodeID)
	fmt.Printf("Status     : %s\n", formatStatus(status.Status))
	fmt.Printf("Uptime     : %s\n", status.Uptime)
	fmt.Printf("Sync Health: %.1f%%\n", status.SyncHealth)
	fmt.Printf("Active Locks: %d\n", status.ActiveLocks)
	fmt.Println()

	fmt.Println("--- Peers ---")
	for _, p := range status.Peers {
		statusIcon := "●"
		if p.Status == "syncing" {
			statusIcon = "◐"
		} else if p.Status == "offline" {
			statusIcon = "○"
		}
		fmt.Printf("  %s %-10s %-15s %4dms  %s\n",
			statusIcon, p.Name, p.ID, p.Latency, p.Transport)
	}

	return nil
}

func runStatusWatch(status *ClusterStatus) error {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	// 초기 출력
	fmt.Print("\033[2J\033[H") // 화면 클리어
	printStatus(status)

	for range ticker.C {
		// TODO: 상태 갱신
		fmt.Print("\033[2J\033[H") // 화면 클리어
		printStatus(status)
	}

	return nil
}

func formatStatus(status string) string {
	switch status {
	case "connected":
		return "● Connected"
	case "connecting":
		return "◐ Connecting..."
	case "disconnected":
		return "○ Disconnected"
	default:
		return status
	}
}
