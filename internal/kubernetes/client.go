package kubernetes

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// NewClient builds a Kubernetes clientset from the first usable kubeconfig.
// Search order:
//  1. KUBECONFIG (supports multiple paths separated by the OS list separator)
//  2. $HOME/.kube/config
//  3. SUDO_USER's ~/.kube/config (when run via sudo)
//  4. /etc/kubernetes/admin.conf (kind control-plane / kubeadm)
//  5. In-cluster config
func NewClient() (*kubernetes.Clientset, error) {
	tried := make([]string, 0, 8)
	for _, p := range kubeconfigCandidates() {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err != nil {
			tried = append(tried, p)
			continue
		}
		config, err := clientcmd.BuildConfigFromFlags("", p)
		if err != nil {
			tried = append(tried, fmt.Sprintf("%s (%v)", p, err))
			continue
		}
		return kubernetes.NewForConfig(config)
	}

	if config, err := rest.InClusterConfig(); err == nil {
		return kubernetes.NewForConfig(config)
	}
	tried = append(tried, "in-cluster")

	return nil, fmt.Errorf("no kubeconfig found in %v", tried)
}

func kubeconfigCandidates() []string {
	var paths []string

	if kc := os.Getenv("KUBECONFIG"); kc != "" {
		for _, p := range filepath.SplitList(kc) {
			p = strings.TrimSpace(p)
			if p != "" {
				paths = append(paths, p)
			}
		}
	}

	if home := homedir.HomeDir(); home != "" {
		paths = append(paths, filepath.Join(home, ".kube", "config"))
	}

	// sudo without -E drops KUBECONFIG and switches $HOME to /root.
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		if u, err := user.Lookup(sudoUser); err == nil && u.HomeDir != "" {
			paths = append(paths, filepath.Join(u.HomeDir, ".kube", "config"))
		}
	}

	paths = append(paths, "/etc/kubernetes/admin.conf")
	return uniqueStrings(paths)
}

func uniqueStrings(in []string) []string {
	seen := make(map[string]struct{}, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return out
}
