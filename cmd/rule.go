package cmd

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"time"

	"Kubernetes-plugin/internal/flow"
	"Kubernetes-plugin/internal/rule"

	"github.com/spf13/cobra"
)

var (
	rulePath      string
	ruleDuration  time.Duration
	ruleInterval  time.Duration
	ruleNoHeaders bool
)

var ruleCmd = &cobra.Command{
	Use:   "rule",
	Short: "Execute user-defined WASM rules",
	Long: `Load and execute user-defined rules written in WebAssembly.
Rules can inspect network flows, throughput, retransmissions, and latency data.
They return pass/fail results and can trigger custom alerts.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if rulePath == "" {
			return fmt.Errorf("--rule is required (path to .wasm file)")
		}

		ctx, cancel := context.WithCancel(cmd.Context())
		defer cancel()

		eng := rule.NewEngine()
		defer eng.Close(ctx)

		fmt.Fprintf(os.Stderr, "Loading rule: %s\n", rulePath)
		if err := eng.Load(ctx, rulePath); err != nil {
			return fmt.Errorf("load rule: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Rule: %s\n", eng.Name())
		fmt.Fprintf(os.Stderr, "Description: %s\n", eng.Description())

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)

		interval := ruleInterval
		if interval <= 0 {
			interval = 5 * time.Second
		}
		if ruleDuration <= 0 {
			ruleDuration = 30 * time.Second
		}

		var log io.Writer = os.Stderr
		if ruleNoHeaders {
			log = io.Discard
		}

		fmt.Fprintf(log, "Evaluating rules for %s (interval: %s)...\n", ruleDuration, interval)

		timer := time.After(ruleDuration)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		evalCount := 0
		alertCount := 0

		for {
			select {
			case <-sig:
				fmt.Fprintf(log, "\nStopped after %d evaluations, %d alerts\n", evalCount, alertCount)
				return nil
			case <-timer:
				fmt.Fprintf(log, "\nCompleted %d evaluations, %d alerts\n", evalCount, alertCount)
				return nil
			case <-ticker.C:
				events, err := collectFlows(ruleDuration / 4)
				if err != nil {
					return fmt.Errorf("collect flows: %w", err)
				}

				ctxData := rule.NewContextFromFlows(events)
				jsonData, err := rule.SerializeContext(ctxData)
				if err != nil {
					return fmt.Errorf("serialize context: %w", err)
				}

				result, err := eng.Evaluate(ctx, jsonData)
				if err != nil {
					return fmt.Errorf("evaluate: %w", err)
				}

				evalCount++
				if result == rule.Alert {
					alertCount++
					for _, a := range eng.Alerts() {
						fmt.Fprintf(os.Stderr, "ALERT [%s]: %s\n", eng.Name(), a)
					}
				} else {
					fmt.Fprintf(log, "PASS [%s] (eval #%d)\n", eng.Name(), evalCount)
				}
			}
		}
	},
}

func collectFlows(d time.Duration) ([]flow.FlowEvent, error) {
	c, err := flow.NewCollector()
	if err != nil {
		if err == flow.ErrNoPrivileges {
			return nil, fmt.Errorf("eBPF privileges required for rule evaluation")
		}
		return nil, err
	}
	defer c.Close()

	var events []flow.FlowEvent
	timer := time.After(d)
	for {
		select {
		case <-timer:
			return events, nil
		default:
			ev, err := c.Read()
			if err != nil {
				return events, nil
			}
			events = append(events, ev)
		}
	}
}

func init() {
	ruleCmd.Flags().StringVar(&rulePath, "rule", "", "Path to .wasm rule file (required)")
	ruleCmd.Flags().DurationVarP(&ruleDuration, "duration", "d", 30*time.Second, "Total evaluation duration")
	ruleCmd.Flags().DurationVarP(&ruleInterval, "interval", "i", 5*time.Second, "Evaluation interval")
	ruleCmd.Flags().BoolVar(&ruleNoHeaders, "no-headers", false, "Suppress progress messages")
	rootCmd.AddCommand(ruleCmd)
}
