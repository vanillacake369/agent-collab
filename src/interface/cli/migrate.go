package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"agent-collab/src/infrastructure/storage"
)

var migrateCmd = &cobra.Command{
	Use:   "migrate",
	Short: "스토리지 마이그레이션 관리",
	Long: `BadgerDB 스토리지 마이그레이션을 관리합니다.

사용 예시:
  agent-collab migrate status     마이그레이션 상태 확인
  agent-collab migrate start      BadgerDB 마이그레이션 시작
  agent-collab migrate rollback   마이그레이션 롤백
  agent-collab migrate backups    백업 목록 확인`,
}

var migrateStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "마이그레이션 상태 확인",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir := getDataDir()
		mgr := storage.NewMigrationManager(dataDir)

		status, err := mgr.Status()
		if err != nil {
			return fmt.Errorf("상태 확인 실패: %w", err)
		}

		fmt.Printf("데이터 디렉토리: %s\n", dataDir)
		fmt.Printf("마이그레이션 상태: %s\n", status)

		// Check BadgerDB directories
		badgerDir := filepath.Join(dataDir, "badger")
		if _, err := os.Stat(badgerDir); err == nil {
			entries, _ := os.ReadDir(badgerDir)
			fmt.Printf("BadgerDB 인스턴스: %d개\n", len(entries))
			for _, e := range entries {
				if e.IsDir() {
					fmt.Printf("  - %s\n", e.Name())
				}
			}
		} else {
			fmt.Println("BadgerDB 디렉토리 없음")
		}

		// List backups
		backups, _ := mgr.ListBackups()
		if len(backups) > 0 {
			fmt.Printf("백업: %d개\n", len(backups))
			for _, b := range backups {
				fmt.Printf("  - %s\n", b)
			}
		}

		return nil
	},
}

var migrateStartCmd = &cobra.Command{
	Use:   "start",
	Short: "BadgerDB 마이그레이션 시작",
	Long: `BadgerDB 스토리지로 마이그레이션을 시작합니다.

마이그레이션 과정:
1. 기존 데이터 백업
2. BadgerDB 테스트
3. BadgerDB 디렉토리 생성
4. 마이그레이션 완료 표시

주의: 마이그레이션 전 백업이 자동으로 생성됩니다.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir := getDataDir()
		mgr := storage.NewMigrationManager(dataDir)

		fmt.Println("BadgerDB 마이그레이션을 시작합니다...")
		fmt.Println()

		if err := mgr.Migrate(); err != nil {
			return fmt.Errorf("마이그레이션 실패: %w", err)
		}

		fmt.Println()
		fmt.Println("마이그레이션이 완료되었습니다!")
		fmt.Println("daemon을 재시작하면 BadgerDB 스토리지가 활성화됩니다.")

		return nil
	},
}

var migrateRollbackCmd = &cobra.Command{
	Use:   "rollback",
	Short: "마이그레이션 롤백",
	Long: `BadgerDB 마이그레이션을 롤백하고 이전 스토리지로 복구합니다.

주의: BadgerDB에 저장된 데이터는 삭제됩니다.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir := getDataDir()
		mgr := storage.NewMigrationManager(dataDir)

		fmt.Println("마이그레이션을 롤백합니다...")
		fmt.Println()

		if err := mgr.Rollback(); err != nil {
			return fmt.Errorf("롤백 실패: %w", err)
		}

		fmt.Println()
		fmt.Println("롤백이 완료되었습니다.")
		fmt.Println("daemon을 재시작하면 기존 스토리지가 사용됩니다.")

		return nil
	},
}

var migrateBackupsCmd = &cobra.Command{
	Use:   "backups",
	Short: "백업 목록 확인",
	RunE: func(cmd *cobra.Command, args []string) error {
		dataDir := getDataDir()
		mgr := storage.NewMigrationManager(dataDir)

		backups, err := mgr.ListBackups()
		if err != nil {
			return fmt.Errorf("백업 목록 조회 실패: %w", err)
		}

		if len(backups) == 0 {
			fmt.Println("백업이 없습니다.")
			return nil
		}

		fmt.Printf("백업 목록 (%d개):\n", len(backups))
		for _, backup := range backups {
			backupPath := filepath.Join(dataDir, backup)
			info, err := os.Stat(backupPath)
			if err == nil {
				fmt.Printf("  %s (생성: %s)\n", backup, info.ModTime().Format("2006-01-02 15:04:05"))
			} else {
				fmt.Printf("  %s\n", backup)
			}
		}

		return nil
	},
}

var restoreBackupName string

var migrateRestoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "백업에서 복구",
	Long: `특정 백업에서 데이터를 복구합니다.

사용 예시:
  agent-collab migrate restore --backup backup_20260209_120000`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if restoreBackupName == "" {
			return fmt.Errorf("--backup 플래그로 백업 이름을 지정하세요")
		}

		dataDir := getDataDir()
		mgr := storage.NewMigrationManager(dataDir)

		fmt.Printf("백업 '%s'에서 복구합니다...\n", restoreBackupName)

		if err := mgr.RestoreBackup(restoreBackupName); err != nil {
			return fmt.Errorf("복구 실패: %w", err)
		}

		fmt.Println("복구가 완료되었습니다.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(migrateCmd)

	migrateCmd.AddCommand(migrateStatusCmd)
	migrateCmd.AddCommand(migrateStartCmd)
	migrateCmd.AddCommand(migrateRollbackCmd)
	migrateCmd.AddCommand(migrateBackupsCmd)
	migrateCmd.AddCommand(migrateRestoreCmd)

	migrateRestoreCmd.Flags().StringVar(&restoreBackupName, "backup", "", "복구할 백업 이름")
}

// getDataDir returns the data directory path.
func getDataDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".agent-collab"
	}
	return filepath.Join(homeDir, ".agent-collab")
}
