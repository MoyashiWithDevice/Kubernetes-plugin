package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "detective",
	Short: "Kubernetes network diagnostic tool",
	Long: `kubectl-detective is a diagnostic platform for Kubernetes cluster communication.
It collects network flows via eBPF and provides visualization, analysis, and monitoring.`,
}

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version number",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("kubectl-detective version 0.1.0")
	},
}

func Execute() {
	rootCmd.AddCommand(versionCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
