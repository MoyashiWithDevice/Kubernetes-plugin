package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/moyashiwithdevice/kubectl-detective/internal/aggregator"

	"github.com/spf13/cobra"
)

var (
	aggregatorAddr string
)

var aggregatorCmd = &cobra.Command{
	Use:   "aggregator",
	Short: "Run the gRPC aggregator server",
	Long: `Start the detective aggregator that receives snapshots from agents
and provides a cluster-wide view of network metrics.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		store := aggregator.NewStore()
		srv := aggregator.NewServer(store)

		fmt.Fprintln(os.Stderr, "Starting aggregator...")
		return srv.Serve(ctx, aggregatorAddr)
	},
}

func init() {
	aggregatorCmd.Flags().StringVar(&aggregatorAddr, "listen", ":50051", "gRPC listen address")
	rootCmd.AddCommand(aggregatorCmd)
}
