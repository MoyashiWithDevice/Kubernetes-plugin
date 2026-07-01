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

var noResolve bool

var flowsCmd = &cobra.Command{
	Use:   "flows",
	Short: "Capture and display TCP flows in real-time",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := flow.NewCollector()
		if err != nil {
			if err == flow.ErrNoPrivileges {
				if noResolve {
					return flow.RunInKind("-n")
				}
				return flow.RunInKind()
			}
			return err
		}
		defer c.Close()

		var r *resolver.PodResolver
		if noResolve {
			fmt.Fprintln(os.Stderr, "resolver: disabled (-n)")
			r = resolver.New(nil, false)
		} else {
			client, err := kubernetes.NewClient()
			if err != nil {
				return fmt.Errorf("kubernetes client: %w", err)
			}
			r = resolver.New(client, true)
			defer r.Close()
		}

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
	flowsCmd.Flags().BoolVarP(&noResolve, "no-resolve", "n", false, "Skip pod name resolution (show IPs only)")
	rootCmd.AddCommand(flowsCmd)
}
