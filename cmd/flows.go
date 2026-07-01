package cmd

import (
	"fmt"
	"os"
	"os/signal"

	"Kubernetes-plugin/internal/flow"
	"Kubernetes-plugin/internal/kubernetes"
	"Kubernetes-plugin/internal/resolver"

	"github.com/spf13/cobra"
)

var (
	noResolve  bool
	resolvePod bool
	resolveSvc bool
)

var flowsCmd = &cobra.Command{
	Use:   "flows",
	Short: "Capture and display TCP flows in real-time",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := flow.NewCollector()
		if err != nil {
			if err == flow.ErrNoPrivileges {
				args := []string{}
				if noResolve {
					args = append(args, "-n")
				}
				if resolvePod {
					args = append(args, "--pod")
				}
				return flow.RunInKind("flows", args...)
			}
			return err
		}
		defer c.Close()

		var r resolver.Resolver
		switch {
		case noResolve:
			fmt.Fprintln(os.Stderr, "resolver: disabled (-n)")
			r = resolver.NewPod(nil, false)
		case resolvePod:
			fmt.Fprintln(os.Stderr, "resolver: pod mode")
			client, err := kubernetes.NewClient()
			if err != nil {
				return fmt.Errorf("kubernetes client: %w", err)
			}
			r = resolver.NewPod(client, true)
		default:
			fmt.Fprintln(os.Stderr, "resolver: service mode")
			client, err := kubernetes.NewClient()
			if err != nil {
				return fmt.Errorf("kubernetes client: %w", err)
			}
			r = resolver.NewService(client, true)
		}
		defer r.Close()

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)

		go func() {
			<-sig
			c.Close()
		}()

		fmt.Fprintln(os.Stderr, "Capturing TCP flows... Press Ctrl+C to stop.")
		for {
			ev, err := c.Read()
			if err != nil {
				return err
			}
			fmt.Printf("%s:%d → %s:%d [pid=%d] (%s)\n",
				r.Resolve(ev.SrcIP), ev.SrcPort,
				r.Resolve(ev.DstIP), ev.DstPort,
				ev.PID, ev.Comm)
		}
	},
}

func init() {
	flowsCmd.Flags().BoolVarP(&noResolve, "no-resolve", "n", false, "Skip name resolution (show IPs only)")
	flowsCmd.Flags().BoolVarP(&resolvePod, "pod", "", false, "Resolve IPs to Pod names")
	flowsCmd.Flags().BoolVarP(&resolveSvc, "svc", "", false, "Resolve IPs to Service names")
	rootCmd.AddCommand(flowsCmd)
}
