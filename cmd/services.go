package cmd

import (
	"context"
	"fmt"

	"Kubernetes-plugin/internal/kubernetes"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "List services in the cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kubernetes.NewClient()
		if err != nil {
			return err
		}
		services, err := client.CoreV1().Services("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		fmt.Printf("%-50s %-20s %-15s %-10s\n", "NAME", "NAMESPACE", "TYPE", "CLUSTER-IP")
		for _, s := range services.Items {
			fmt.Printf("%-50s %-20s %-15s %-10s\n", s.Name, s.Namespace, string(s.Spec.Type), s.Spec.ClusterIP)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(servicesCmd)
}
