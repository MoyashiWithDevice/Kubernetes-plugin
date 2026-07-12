package cmd

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"Kubernetes-plugin/internal/export"
	"Kubernetes-plugin/internal/flow"
	"Kubernetes-plugin/internal/graph"
	"Kubernetes-plugin/internal/kubernetes"
	"Kubernetes-plugin/internal/resolver"

	"github.com/spf13/cobra"
)

var (
	mapNoResolve  bool
	mapResolvePod bool
	mapResolveSvc bool
	mapDuration   time.Duration
	mapNoHeaders  bool
	mapFormat     string
	mapOutput     string
)

var mapCmd = &cobra.Command{
	Use:   "map",
	Short: "Show service dependency map",
	Long: `Collect TCP flows and display a service dependency map.
Default output is ASCII art.
Use --format to export as csv, json, html, mermaid, or ascii.
Use --output (-o) to write to a file instead of stdout.
Use --no-headers to suppress progress messages (useful for file redirect).`,
	RunE: func(cmd *cobra.Command, args []string) error {
		c, err := flow.NewCollector()
		if err != nil {
			if err == flow.ErrNoPrivileges {
				var extra []string
				if mapNoResolve {
					extra = append(extra, "-n")
				}
				if mapResolvePod {
					extra = append(extra, "--pod")
				}
				if mapDuration > 0 {
					extra = append(extra, "--duration", mapDuration.String())
				}
				if mapNoHeaders {
					extra = append(extra, "--no-headers")
				}
				if mapFormat != "" {
					extra = append(extra, "--format", mapFormat)
				}
				if mapOutput != "" {
					f, err := os.Create(mapOutput)
					if err != nil {
						return fmt.Errorf("create output file: %w", err)
					}
					defer f.Close()
					return flow.RunInKindTo("map", f, extra...)
				}
				return flow.RunInKind("map", extra...)
			}
			return err
		}
		defer c.Close()

		var log io.Writer = os.Stderr
		if mapNoHeaders {
			log = io.Discard
			resolver.SetLogOutput(io.Discard)
		}

		var r resolver.Resolver
		switch {
		case mapNoResolve:
			fmt.Fprintln(log, "resolver: disabled (-n)")
			r = resolver.NewPod(nil, false)
		case mapResolvePod:
			fmt.Fprintln(log, "resolver: pod mode")
			client, err := kubernetes.NewClient()
			if err != nil {
				return fmt.Errorf("kubernetes client: %w", err)
			}
			r = resolver.NewPod(client, true)
		default:
			fmt.Fprintln(log, "resolver: service mode")
			client, err := kubernetes.NewClient()
			if err != nil {
				return fmt.Errorf("kubernetes client: %w", err)
			}
			r = resolver.NewService(client, true)
		}
		defer r.Close()

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)

		g := graph.New()

		if mapDuration > 0 {
			fmt.Fprintf(log, "Collecting flows for %s...\n", mapDuration)
			timer := time.After(mapDuration)
		loop:
			for {
				select {
				case <-timer:
					break loop
				case <-sig:
					break loop
				default:
				}
				ev, err := c.Read()
				if err != nil {
					return err
				}
				g.AddEdge(
					r.Resolve(ev.SrcIP),
					r.Resolve(ev.DstIP),
				)
			}
		} else {
			fmt.Fprintln(log, "Collecting flows... Press Ctrl+C to stop.")
			go func() {
				<-sig
				c.Close()
			}()
			for {
				ev, err := c.Read()
				if err != nil {
					return err
				}
				g.AddEdge(
					r.Resolve(ev.SrcIP),
					r.Resolve(ev.DstIP),
				)
			}
		}

		format := resolveMapFormat()

		rpt := export.NewReport(g, mapDuration)

		var out io.Writer = os.Stdout
		if mapOutput != "" {
			f, err := os.Create(mapOutput)
			if err != nil {
				return fmt.Errorf("create output file: %w", err)
			}
			defer f.Close()
			out = f
			fmt.Fprintf(log, "Writing %s to %s\n", format, mapOutput)
		}

		switch format {
		case "csv":
			return export.WriteCSV(out, rpt)
		case "json":
			return export.WriteJSON(out, rpt)
		case "html":
			return export.WriteHTML(out, rpt)
		case "mermaid":
			_, err := fmt.Fprint(out, g.Mermaid())
			return err
		default:
			if !mapNoHeaders && mapOutput == "" {
				fmt.Fprint(out, "Service Map\n═══════════\n\n")
			}
			_, err := fmt.Fprint(out, g.ASCII())
			return err
		}
	},
}

func resolveMapFormat() string {
	f := strings.ToLower(mapFormat)
	if f != "" {
		return f
	}
	return "ascii"
}

func init() {
	mapCmd.Flags().StringVarP(&mapFormat, "format", "f", "", "Output format: ascii, mermaid, csv, json, html (default: ascii)")
	mapCmd.Flags().StringVarP(&mapOutput, "output", "o", "", "Write output to file instead of stdout")
	mapCmd.Flags().BoolVarP(&mapNoHeaders, "no-headers", "", false, "Suppress progress messages")
	mapCmd.Flags().BoolVarP(&mapNoResolve, "no-resolve", "n", false, "Skip name resolution (show IPs only)")
	mapCmd.Flags().BoolVarP(&mapResolvePod, "pod", "", false, "Resolve IPs to Pod names")
	mapCmd.Flags().BoolVarP(&mapResolveSvc, "svc", "", false, "Resolve IPs to Service names")
	mapCmd.Flags().DurationVarP(&mapDuration, "duration", "d", 10*time.Second, "Collection duration (0 = continuous)")
	rootCmd.AddCommand(mapCmd)
}
