package cmd

import (
	"context"
	"fmt"

	"Kubernetes-plugin/internal/kubernetes"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var endpointSlicesCmd = &cobra.Command{
	Use:   "endpointslices",
	Short: "List EndpointSlices in the cluster",
	RunE: func(cmd *cobra.Command, args []string) error {
		client, err := kubernetes.NewClient()
		if err != nil {
			return err
		}
		eps, err := client.DiscoveryV1().EndpointSlices("").List(context.Background(), metav1.ListOptions{})
		if err != nil {
			return err
		}
		fmt.Printf("%-50s %-20s %-10s\n", "NAME", "NAMESPACE", "ENDPOINTS")
		for _, ep := range eps.Items {
			addrs := ""
			for i, e := range ep.Endpoints {
				if i > 0 {
					addrs += ", "
				}
				if len(e.Addresses) > 0 {
					addrs += e.Addresses[0]
				}
			}
			if len(addrs) > 50 {
				addrs = addrs[:47] + "..."
			}
			fmt.Printf("%-50s %-20s %-10s\n", ep.Name, ep.Namespace, addrs)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(endpointSlicesCmd)
}
