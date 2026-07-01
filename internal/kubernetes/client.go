package kubernetes

import (
	"fmt"
	"os"
	"path/filepath"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

func NewClient() (*kubernetes.Clientset, error) {
	paths := []string{
		os.Getenv("KUBECONFIG"),
		filepath.Join(homedir.HomeDir(), ".kube", "config"),
		"/etc/kubernetes/admin.conf",
	}
	for _, p := range paths {
		if p == "" {
			continue
		}
		config, err := clientcmd.BuildConfigFromFlags("", p)
		if err == nil {
			return kubernetes.NewForConfig(config)
		}
	}
	return nil, fmt.Errorf("no kubeconfig found in %v", paths)
}
