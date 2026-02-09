package cli

import (
	"fmt"

	"agent-collab/internal/infrastructure/network/wireguard"
	"agent-collab/internal/infrastructure/network/wireguard/platform"

	"github.com/spf13/cobra"
)

var wireguardCmd = &cobra.Command{
	Use:   "wireguard",
	Short: "WireGuard VPN 관리",
	Long:  `WireGuard VPN 인터페이스 상태 확인 및 관리`,
}

var wgStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "WireGuard 인터페이스 상태 확인",
	RunE:  runWGStatus,
}

var wgPeersCmd = &cobra.Command{
	Use:   "peers",
	Short: "WireGuard 피어 목록",
	RunE:  runWGPeers,
}

var wgSupportCmd = &cobra.Command{
	Use:   "support",
	Short: "WireGuard 지원 여부 확인",
	RunE:  runWGSupport,
}

func init() {
	rootCmd.AddCommand(wireguardCmd)
	wireguardCmd.AddCommand(wgStatusCmd)
	wireguardCmd.AddCommand(wgPeersCmd)
	wireguardCmd.AddCommand(wgSupportCmd)
}

func runWGStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("WireGuard 인터페이스 상태")
	fmt.Println("========================")
	fmt.Println()

	// Check platform support
	p := platform.Get()
	if !p.IsSupported() {
		fmt.Println("⚠ WireGuard가 이 시스템에서 지원되지 않습니다.")
		fmt.Printf("  플랫폼: %s\n", p.Name())
		return nil
	}

	fmt.Printf("플랫폼: %s\n", p.Name())
	fmt.Printf("지원: ✓\n")

	if p.RequiresRoot() {
		fmt.Println("권한: 관리자 권한 필요")
	} else {
		fmt.Println("권한: ✓ (현재 관리자)")
	}

	// Get external IP
	if ip, err := p.GetExternalIP(); err == nil {
		fmt.Printf("외부 IP: %s\n", ip)
	}

	fmt.Println()
	fmt.Println("인터페이스 상태를 확인하려면 클러스터에 참여해야 합니다.")

	return nil
}

func runWGPeers(cmd *cobra.Command, args []string) error {
	fmt.Println("WireGuard 피어 목록")
	fmt.Println("==================")
	fmt.Println()

	// Check platform support
	p := platform.Get()
	if !p.IsSupported() {
		fmt.Println("⚠ WireGuard가 이 시스템에서 지원되지 않습니다.")
		return nil
	}

	fmt.Println("피어 목록을 확인하려면 클러스터에 참여해야 합니다.")
	fmt.Println()
	fmt.Println("사용 방법:")
	fmt.Println("  1. agent-collab init -p <project> --wireguard  (클러스터 생성)")
	fmt.Println("  2. agent-collab join <token>                   (클러스터 참여)")
	fmt.Println("  3. agent-collab wireguard peers                (피어 목록 확인)")

	return nil
}

func runWGSupport(cmd *cobra.Command, args []string) error {
	fmt.Println("WireGuard 지원 확인")
	fmt.Println("==================")
	fmt.Println()

	p := platform.Get()

	fmt.Printf("플랫폼: %s\n", p.Name())

	if p.IsSupported() {
		fmt.Println("상태: ✓ 지원됨")

		if p.RequiresRoot() {
			fmt.Println("권한: 관리자 권한 필요")
			fmt.Println()
			fmt.Println("WireGuard 사용을 위해 관리자 권한으로 실행하세요:")
			fmt.Println("  sudo agent-collab init -p <project> --wireguard")
		} else {
			fmt.Println("권한: ✓ 현재 관리자 권한 보유")
		}

		// Get external IP
		if ip, err := p.GetExternalIP(); err == nil {
			fmt.Printf("외부 IP: %s\n", ip)
		}
	} else {
		fmt.Println("상태: ✗ 지원되지 않음")
		fmt.Println()
		fmt.Println("WireGuard 설치가 필요합니다:")
		switch p.Name() {
		case "linux":
			fmt.Println("  apt install wireguard-tools   # Debian/Ubuntu")
			fmt.Println("  dnf install wireguard-tools   # Fedora")
			fmt.Println("  pacman -S wireguard-tools     # Arch")
		case "darwin":
			fmt.Println("  brew install wireguard-go wireguard-tools")
		case "windows":
			fmt.Println("  https://www.wireguard.com/install/ 에서 설치")
		}
	}

	fmt.Println()
	fmt.Println("WireGuard 패키지 정보:")
	fmt.Printf("  wireguard-go: %s\n", wireguard.Version)

	return nil
}
