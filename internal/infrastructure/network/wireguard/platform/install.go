package platform

import (
	"fmt"
	"runtime"
	"strings"
)

// InstallInfo contains installation instructions for WireGuard.
type InstallInfo struct {
	OS           string
	Arch         string
	Instructions []string
	URLs         []string
}

// GetInstallInstructions returns platform-specific installation instructions.
func GetInstallInstructions() *InstallInfo {
	info := &InstallInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	switch runtime.GOOS {
	case "darwin":
		info.Instructions = getDarwinInstructions(runtime.GOARCH)
		info.URLs = []string{"https://www.wireguard.com/install/"}
	case "linux":
		info.Instructions = getLinuxInstructions(runtime.GOARCH)
		info.URLs = []string{"https://www.wireguard.com/install/"}
	case "windows":
		info.Instructions = getWindowsInstructions(runtime.GOARCH)
		info.URLs = []string{
			"https://www.wireguard.com/install/",
			"https://download.wireguard.com/windows-client/",
		}
	default:
		info.Instructions = []string{
			fmt.Sprintf("WireGuard is not officially supported on %s/%s", runtime.GOOS, runtime.GOARCH),
			"Please visit https://www.wireguard.com for more information",
		}
		info.URLs = []string{"https://www.wireguard.com"}
	}

	return info
}

func getDarwinInstructions(arch string) []string {
	instructions := []string{
		"WireGuard를 설치하려면 다음 명령어를 실행하세요:",
		"",
	}

	switch arch {
	case "arm64":
		instructions = append(instructions,
			"  # Homebrew 사용 (Apple Silicon)",
			"  brew install wireguard-go wireguard-tools",
			"",
			"  # 또는 MacPorts 사용",
			"  sudo port install wireguard-go wireguard-tools",
		)
	case "amd64":
		instructions = append(instructions,
			"  # Homebrew 사용 (Intel)",
			"  brew install wireguard-go wireguard-tools",
			"",
			"  # 또는 MacPorts 사용",
			"  sudo port install wireguard-go wireguard-tools",
		)
	default:
		instructions = append(instructions,
			"  # Homebrew 사용",
			"  brew install wireguard-go wireguard-tools",
		)
	}

	instructions = append(instructions,
		"",
		"설치 후 다시 시도하세요.",
	)

	return instructions
}

func getLinuxInstructions(arch string) []string {
	instructions := []string{
		"WireGuard를 설치하려면 배포판에 맞는 명령어를 실행하세요:",
		"",
	}

	// Common distro instructions
	distroInstructions := []string{
		"  # Ubuntu/Debian",
		"  sudo apt update && sudo apt install wireguard wireguard-tools",
		"",
		"  # Fedora",
		"  sudo dnf install wireguard-tools",
		"",
		"  # CentOS/RHEL 8+",
		"  sudo dnf install epel-release elrepo-release",
		"  sudo dnf install kmod-wireguard wireguard-tools",
		"",
		"  # Arch Linux",
		"  sudo pacman -S wireguard-tools",
		"",
		"  # openSUSE",
		"  sudo zypper install wireguard-tools",
		"",
		"  # Alpine",
		"  sudo apk add wireguard-tools",
	}

	instructions = append(instructions, distroInstructions...)

	// Architecture-specific notes
	switch arch {
	case "arm64", "arm":
		instructions = append(instructions,
			"",
			"  # ARM 장치 참고사항:",
			"  # 커널 버전 5.6 이상에서는 WireGuard가 내장되어 있습니다.",
			"  # 이전 커널에서는 wireguard-dkms가 필요할 수 있습니다.",
		)
	}

	instructions = append(instructions,
		"",
		"설치 후 다시 시도하세요.",
	)

	return instructions
}

func getWindowsInstructions(arch string) []string {
	instructions := []string{
		"WireGuard를 설치하려면:",
		"",
	}

	switch arch {
	case "amd64":
		instructions = append(instructions,
			"  # 방법 1: 공식 설치 프로그램 (권장)",
			"  # https://download.wireguard.com/windows-client/wireguard-installer.exe",
			"",
			"  # 방법 2: winget 사용",
			"  winget install WireGuard.WireGuard",
			"",
			"  # 방법 3: Chocolatey 사용",
			"  choco install wireguard",
			"",
			"  # 방법 4: Scoop 사용",
			"  scoop install wireguard",
		)
	case "arm64":
		instructions = append(instructions,
			"  # Windows ARM64용 WireGuard",
			"  # https://download.wireguard.com/windows-client/ 에서 ARM64 버전 다운로드",
			"",
			"  # 또는 winget 사용",
			"  winget install WireGuard.WireGuard",
		)
	default:
		instructions = append(instructions,
			"  # 공식 설치 프로그램 다운로드",
			"  # https://download.wireguard.com/windows-client/wireguard-installer.exe",
		)
	}

	instructions = append(instructions,
		"",
		"설치 후 관리자 권한으로 다시 시도하세요.",
	)

	return instructions
}

// FormatInstallInstructions returns a formatted string of installation instructions.
func FormatInstallInstructions() string {
	info := GetInstallInstructions()
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("플랫폼: %s/%s\n", info.OS, info.Arch))
	sb.WriteString("\n")

	for _, line := range info.Instructions {
		sb.WriteString(line)
		sb.WriteString("\n")
	}

	if len(info.URLs) > 0 {
		sb.WriteString("\n참고 링크:\n")
		for _, url := range info.URLs {
			sb.WriteString(fmt.Sprintf("  - %s\n", url))
		}
	}

	return sb.String()
}

// CheckAndSuggestInstall checks if WireGuard is supported and returns installation suggestion if not.
func CheckAndSuggestInstall() (supported bool, suggestion string) {
	p := GetPlatform()
	if p.IsSupported() {
		return true, ""
	}

	var sb strings.Builder
	sb.WriteString("⚠ WireGuard가 설치되어 있지 않습니다.\n\n")
	sb.WriteString(FormatInstallInstructions())

	return false, sb.String()
}
