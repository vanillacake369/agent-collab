package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "버전 정보",
	Long:  `agent-collab 버전 정보를 출력합니다.`,
	Run:   runVersion,
}

var versionShort bool

func init() {
	rootCmd.AddCommand(versionCmd)
	versionCmd.Flags().BoolVarP(&versionShort, "short", "s", false, "버전만 출력")
}

func runVersion(cmd *cobra.Command, args []string) {
	v, c, d, b := GetVersionInfo()

	if versionShort {
		fmt.Println(v)
		return
	}

	fmt.Printf("agent-collab %s\n", v)
	fmt.Printf("  Commit:    %s\n", c)
	fmt.Printf("  Built:     %s\n", d)
	fmt.Printf("  Built by:  %s\n", b)
	fmt.Printf("  Go:        %s\n", runtime.Version())
	fmt.Printf("  OS/Arch:   %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
