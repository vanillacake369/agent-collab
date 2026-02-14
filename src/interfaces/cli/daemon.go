package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"time"

	"agent-collab/src/application"
	"agent-collab/src/interfaces/daemon"

	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "데몬 관리",
	Long:  `agent-collab 백그라운드 데몬을 관리합니다.`,
}

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "데몬 시작",
	Long: `백그라운드에서 agent-collab 데몬을 시작합니다.

데몬이 실행되면 MCP 서버와 다른 CLI 명령이
동일한 클러스터 연결을 공유할 수 있습니다.`,
	RunE: runDaemonStart,
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "데몬 중지",
	RunE:  runDaemonStop,
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "데몬 상태",
	RunE:  runDaemonStatus,
}

var daemonRunCmd = &cobra.Command{
	Use:    "run",
	Short:  "데몬 실행 (포그라운드)",
	Hidden: true, // Internal use
	RunE:   runDaemonRun,
}

var (
	daemonForeground bool
	daemonStopAll    bool
)

func init() {
	rootCmd.AddCommand(daemonCmd)

	daemonCmd.AddCommand(daemonStartCmd)
	daemonCmd.AddCommand(daemonStopCmd)
	daemonCmd.AddCommand(daemonStatusCmd)
	daemonCmd.AddCommand(daemonRunCmd)

	daemonStartCmd.Flags().BoolVarP(&daemonForeground, "foreground", "f", false, "포그라운드에서 실행")
	daemonStopCmd.Flags().BoolVar(&daemonStopAll, "all", false, "모든 agent-collab 데몬 프로세스 종료")
}

// ensureDaemonRunning checks if daemon is running and starts it if not.
// This is used by commands that require a running daemon.
func ensureDaemonRunning() error {
	client := daemon.NewClient()

	if client.IsRunning() {
		return nil
	}

	fmt.Println("📡 데몬이 실행 중이 아닙니다. 자동으로 시작합니다...")
	fmt.Println()

	return startDaemonBackground()
}

// startDaemonBackground starts the daemon in background mode.
func startDaemonBackground() error {
	client := daemon.NewClient()

	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("실행 파일 경로를 찾을 수 없습니다: %w", err)
	}

	// #nosec G204 - executable is from os.Executable(), not user input
	daemonProcess := exec.Command(executable, "daemon", "run")
	daemonProcess.Stdout = nil
	daemonProcess.Stderr = nil
	daemonProcess.Stdin = nil

	// Detach from parent process (platform-specific)
	setSysProcAttr(daemonProcess)

	if err := daemonProcess.Start(); err != nil {
		return fmt.Errorf("데몬 시작 실패: %w", err)
	}

	fmt.Printf("🚀 데몬 시작 중... (PID: %d)\n", daemonProcess.Process.Pid)

	// Wait for daemon to be ready
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if client.IsRunning() {
			fmt.Println("✓ 데몬이 시작되었습니다.")
			return nil
		}
	}

	return fmt.Errorf("데몬 시작 시간 초과")
}

func runDaemonStart(cmd *cobra.Command, args []string) error {
	client := daemon.NewClient()

	// Check if already running
	if client.IsRunning() {
		status, _ := client.Status()
		if status != nil && status.ProjectName != "" {
			// Daemon is running with a valid project
			fmt.Println("✓ 데몬이 이미 실행 중입니다.")
			fmt.Printf("  PID: %d\n", status.PID)
			fmt.Printf("  Project: %s\n", status.ProjectName)
			return nil
		}
		// Daemon is running but has no project - restart it
		fmt.Println("⚠ 데몬이 실행 중이지만 프로젝트가 설정되지 않았습니다. 재시작합니다...")
		if err := client.Shutdown(); err != nil {
			// Force kill if shutdown fails
			if pid, err := client.GetPID(); err == nil {
				signalTerm(pid)
			}
		}
		// Wait for daemon to stop
		for i := 0; i < 30; i++ {
			time.Sleep(100 * time.Millisecond)
			if !client.IsRunning() {
				break
			}
		}
	}

	if daemonForeground {
		return runDaemonRun(cmd, args)
	}

	return startDaemonBackground()
}

func runDaemonStop(cmd *cobra.Command, args []string) error {
	// --all 플래그: 모든 agent-collab 데몬 프로세스 종료
	if daemonStopAll {
		return stopAllDaemons()
	}

	client := daemon.NewClient()

	if !client.IsRunning() {
		fmt.Println("데몬이 실행 중이 아닙니다.")
		return nil
	}

	fmt.Println("🛑 데몬 중지 중...")

	if err := client.Shutdown(); err != nil {
		// Try to get PID and terminate
		if pid, err := client.GetPID(); err == nil {
			signalTerm(pid)
		}
	}

	// Wait for daemon to stop
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if !client.IsRunning() {
			fmt.Println("✓ 데몬이 중지되었습니다.")
			return nil
		}
	}

	fmt.Println("⚠️  데몬이 응답하지 않습니다. 강제 종료를 시도합니다.")

	if pid, err := client.GetPID(); err == nil {
		process, err := os.FindProcess(pid)
		if err == nil {
			process.Kill()
		}
	}

	return nil
}

func runDaemonStatus(cmd *cobra.Command, args []string) error {
	client := daemon.NewClient()

	if !client.IsRunning() {
		fmt.Println("● 데몬 상태: 중지됨")
		fmt.Println()
		fmt.Println("데몬을 시작하려면: agent-collab daemon start")
		return nil
	}

	status, err := client.Status()
	if err != nil {
		return fmt.Errorf("상태 조회 실패: %w", err)
	}

	fmt.Println("● 데몬 상태: 실행 중")
	fmt.Println()
	fmt.Printf("  %-16s: %d\n", "PID", status.PID)
	fmt.Printf("  %-16s: %s\n", "시작 시간", status.StartedAt.Format(time.RFC3339))
	fmt.Printf("  %-16s: %s\n", "프로젝트", status.ProjectName)
	fmt.Printf("  %-16s: %s\n", "Node ID", status.NodeID)
	fmt.Printf("  %-16s: %d\n", "연결된 Peer", status.PeerCount)
	fmt.Printf("  %-16s: %d\n", "활성 Lock", status.LockCount)
	fmt.Printf("  %-16s: %d\n", "연결된 Agent", status.AgentCount)
	fmt.Printf("  %-16s: %d\n", "이벤트 구독자", status.EventSubscribers)
	fmt.Printf("  %-16s: %s\n", "Embedding 제공자", status.EmbeddingProvider)

	return nil
}

func runDaemonRun(cmd *cobra.Command, args []string) error {
	// Create application
	app, err := application.New(nil)
	if err != nil {
		return fmt.Errorf("앱 생성 실패: %w", err)
	}

	// Try to load existing configuration (it's OK if it doesn't exist yet)
	ctx := context.Background()
	if err := app.LoadFromConfig(ctx); err != nil {
		// Config doesn't exist - daemon starts without a cluster
		// init/join commands will trigger through daemon API
		fmt.Fprintf(os.Stderr, "No existing config found, daemon starting without cluster\n")
	}

	// Create and start daemon server
	server := daemon.NewServer(app)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := server.Start(ctx); err != nil {
		return fmt.Errorf("데몬 시작 실패: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Daemon started (PID: %d)\n", os.Getpid())

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	<-sigCh
	fmt.Fprintf(os.Stderr, "Shutting down daemon...\n")

	server.Stop()
	return nil
}
