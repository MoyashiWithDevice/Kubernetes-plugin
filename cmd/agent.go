package cmd

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"time"

	"github.com/moyashiwithdevice/kubectl-detective/internal/agent"
	"github.com/moyashiwithdevice/kubectl-detective/internal/flow"

	"github.com/spf13/cobra"
)

var (
	agentNodeName   string
	agentInterval   time.Duration
	agentAggregator string
)

var agentCmd = &cobra.Command{
	Use:   "agent",
	Short: "Run the eBPF agent on a node",
	Long: `Start the detective agent that collects eBPF metrics on this node
and sends periodic snapshots to the aggregator.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if agentAggregator == "" {
			return fmt.Errorf("--aggregator address is required (e.g. detective-aggregator:50051)")
		}

		name := agentNodeName
		if name == "" {
			name = agent.DefaultNodeName()
		}

		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
		defer cancel()

		a := agent.New(name, agentInterval, agentAggregator)
		err := a.Run(ctx)
		if err != nil && errors.Is(err, flow.ErrNoPrivileges) {
			// If the aggregator is on a loopback address, it is unreachable from
			// inside kind containers. Deploy both aggregator and agent inside the
			// same kind node so they can talk via localhost.
			if isLoopbackAddr(agentAggregator) {
				return flow.RunAgentInKind(name, agentInterval, agentAggregator)
			}
			var extra []string
			if agentNodeName != "" {
				extra = append(extra, "--node", agentNodeName)
			}
			if agentInterval > 0 {
				extra = append(extra, "--interval", agentInterval.String())
			}
			if agentAggregator != "" {
				extra = append(extra, "--aggregator", agentAggregator)
			}
			return flow.RunInKind("agent", extra...)
		}
		return err
	},
}

func init() {
	agentCmd.Flags().StringVar(&agentNodeName, "node", "", "Node name (default: hostname)")
	agentCmd.Flags().DurationVar(&agentInterval, "interval", 5*time.Second, "Snapshot interval")
	agentCmd.Flags().StringVar(&agentAggregator, "aggregator", "", "Aggregator gRPC address (host:port)")
	rootCmd.AddCommand(agentCmd)
}

func isLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" || host == "[::1]" || host == "" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
