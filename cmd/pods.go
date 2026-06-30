package cmd

import (
	"context"
	"fmt"

	"Kubernetes-plugin/internal/kubernetes"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var podsCmd = &cobra.Command{
	Use:   "pods",
	Short: "List pods in the cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kubernetes.NewClient()
		if err != nil {
			return err
		}
		pods, err := client.CoreV1().Pods("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		fmt.Printf("%-50s %-20s %-15s %-10s\n", "NAME", "NAMESPACE", "NODE", "STATUS")
		for _, p := range pods.Items {
			fmt.Printf("%-50s %-20s %-15s %-10s\n", p.Name, p.Namespace, p.Spec.NodeName, string(p.Status.Phase))
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(podsCmd)
}
