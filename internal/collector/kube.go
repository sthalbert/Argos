package collector

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeClient talks to a single Kubernetes cluster via client-go. It
// satisfies KubeSource (ServerVersion + ListNodes). The target cluster is
// whatever the loaded kubeconfig points at.
type KubeClient struct {
	clientset *kubernetes.Clientset
}

// NewKubeClient constructs a client. Resolution order:
//   - explicit kubeconfigPath when non-empty;
//   - in-cluster config when running inside a pod;
//   - the default kubectl loading rules (KUBECONFIG env var, then ~/.kube/config).
func NewKubeClient(kubeconfigPath string) (*KubeClient, error) {
	cfg, err := loadKubeConfig(kubeconfigPath)
	if err != nil {
		return nil, err
	}
	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("kubernetes.NewForConfig: %w", err)
	}
	return &KubeClient{clientset: cs}, nil
}

func loadKubeConfig(kubeconfigPath string) (*rest.Config, error) {
	if kubeconfigPath != "" {
		cfg, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("build config from %q: %w", kubeconfigPath, err)
		}
		return cfg, nil
	}
	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("load default kubeconfig: %w", err)
	}
	return cfg, nil
}

// ServerVersion returns the cluster's reported git version (e.g., "v1.29.5").
// client-go's discovery.ServerVersion() does not accept a context, so the
// parameter is retained for interface compatibility but unused.
func (k *KubeClient) ServerVersion(_ context.Context) (string, error) {
	info, err := k.clientset.Discovery().ServerVersion()
	if err != nil {
		return "", err
	}
	return info.GitVersion, nil
}

// ListNodes returns every Node visible through the configured kubeconfig.
// A single List call is used; paginating via Continue is unnecessary at
// the cluster-wide node counts this project targets.
func (k *KubeClient) ListNodes(ctx context.Context) ([]NodeInfo, error) {
	list, err := k.clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list nodes: %w", err)
	}
	out := make([]NodeInfo, 0, len(list.Items))
	for _, n := range list.Items {
		out = append(out, NodeInfo{
			Name:           n.Name,
			KubeletVersion: n.Status.NodeInfo.KubeletVersion,
			OsImage:        n.Status.NodeInfo.OSImage,
			Architecture:   n.Status.NodeInfo.Architecture,
			Labels:         n.Labels,
		})
	}
	return out, nil
}
