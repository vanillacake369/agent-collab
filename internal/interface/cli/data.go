package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var dataCmd = &cobra.Command{
	Use:   "data",
	Short: "데이터 관리",
	Long:  `agent-collab 데이터 디렉토리를 관리합니다.`,
}

var (
	purgeForce       bool
	purgeKeepConfig  bool
	purgeKeepBackups bool
)

var dataPurgeCmd = &cobra.Command{
	Use:   "purge",
	Short: "모든 데이터 삭제",
	Long: `agent-collab의 모든 데이터를 삭제합니다.

삭제되는 항목:
  - BadgerDB 데이터 (delta, audit)
  - Vector 데이터
  - Metrics 데이터
  - 백업 파일
  - 설정 및 키 파일

옵션:
  --keep-config   설정 파일(config.json, key.json) 유지
  --keep-backups  백업 디렉토리 유지
  --force         확인 없이 삭제`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir := getDataDir()

		// Check if directory exists
		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			fmt.Println("데이터 디렉토리가 존재하지 않습니다.")
			return nil
		}

		// Show what will be deleted
		fmt.Printf("데이터 디렉토리: %s\n\n", dataDir)
		fmt.Println("삭제될 항목:")

		entries, err := os.ReadDir(dataDir)
		if err != nil {
			return fmt.Errorf("디렉토리 읽기 실패: %w", err)
		}

		var toDelete []string
		var toKeep []string

		for _, entry := range entries {
			name := entry.Name()

			// Check if should keep
			if purgeKeepConfig && (name == "config.json" || name == "key.json") {
				toKeep = append(toKeep, name)
				continue
			}
			if purgeKeepBackups && strings.HasPrefix(name, "backup_") {
				toKeep = append(toKeep, name)
				continue
			}

			toDelete = append(toDelete, name)
			fmt.Printf("  - %s\n", name)
		}

		if len(toDelete) == 0 {
			fmt.Println("  (삭제할 항목 없음)")
			return nil
		}

		if len(toKeep) > 0 {
			fmt.Println("\n유지될 항목:")
			for _, name := range toKeep {
				fmt.Printf("  - %s\n", name)
			}
		}

		// Confirm deletion
		if !purgeForce {
			fmt.Print("\n정말 삭제하시겠습니까? [y/N]: ")
			reader := bufio.NewReader(os.Stdin)
			response, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("입력 읽기 실패: %w", err)
			}

			response = strings.TrimSpace(strings.ToLower(response))
			if response != "y" && response != "yes" {
				fmt.Println("취소되었습니다.")
				return nil
			}
		}

		// Delete items
		fmt.Println()
		for _, name := range toDelete {
			path := filepath.Join(dataDir, name)
			if err := os.RemoveAll(path); err != nil {
				fmt.Printf("  ✗ %s 삭제 실패: %v\n", name, err)
			} else {
				fmt.Printf("  ✓ %s 삭제됨\n", name)
			}
		}

		// Remove directory if empty
		remaining, _ := os.ReadDir(dataDir)
		if len(remaining) == 0 {
			if err := os.Remove(dataDir); err == nil {
				fmt.Printf("\n데이터 디렉토리 삭제됨: %s\n", dataDir)
			}
		}

		fmt.Println("\n삭제가 완료되었습니다.")
		return nil
	},
}

var dataPathCmd = &cobra.Command{
	Use:   "path",
	Short: "데이터 디렉토리 경로 출력",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println(getDataDir())
	},
}

var dataInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "데이터 사용량 정보",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir := getDataDir()

		if _, err := os.Stat(dataDir); os.IsNotExist(err) {
			fmt.Println("데이터 디렉토리가 존재하지 않습니다.")
			return nil
		}

		fmt.Printf("데이터 디렉토리: %s\n\n", dataDir)

		entries, err := os.ReadDir(dataDir)
		if err != nil {
			return fmt.Errorf("디렉토리 읽기 실패: %w", err)
		}

		var totalSize int64
		fmt.Println("항목별 크기:")

		for _, entry := range entries {
			path := filepath.Join(dataDir, entry.Name())
			size, err := getDirSize(path)
			if err != nil {
				continue
			}
			totalSize += size
			fmt.Printf("  %-25s %s\n", entry.Name(), formatSize(size))
		}

		fmt.Printf("\n총 사용량: %s\n", formatSize(totalSize))
		return nil
	},
}

func getDirSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func init() {
	rootCmd.AddCommand(dataCmd)

	dataCmd.AddCommand(dataPurgeCmd)
	dataCmd.AddCommand(dataPathCmd)
	dataCmd.AddCommand(dataInfoCmd)

	dataPurgeCmd.Flags().BoolVarP(&purgeForce, "force", "f", false, "확인 없이 삭제")
	dataPurgeCmd.Flags().BoolVar(&purgeKeepConfig, "keep-config", false, "설정 파일 유지")
	dataPurgeCmd.Flags().BoolVar(&purgeKeepBackups, "keep-backups", false, "백업 유지")
}
