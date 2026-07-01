package cmd

import (
	"fmt"
	"os"
	"os/signal"

	"Kubernetes-plugin/internal/flow"

	"github.com/spf13/cobra"
)

var flowsCmd = &cobra.Command{
	Use:   "flows",
	Short: "Capture and display TCP flows in real-time",
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := flow.NewCollector()
		if err != nil {
			if err == flow.ErrNoPrivileges {
				return flow.RunInKind()
			}
			return err
		}
		defer c.Close()

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
				ev.SrcIP, ev.SrcPort, ev.DstIP, ev.DstPort, ev.PID, ev.Comm)
		}
	},
}

func init() {
	rootCmd.AddCommand(flowsCmd)
}
