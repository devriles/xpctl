package kube

import (
	"fmt"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type Client struct {
	Dynamic   dynamic.Interface
	Discovery discovery.DiscoveryInterface
	Config    *rest.Config
}

func NewClient(kubeconfigPath, contextOverride string) (*Client, error) {
	var loadingRules *clientcmd.ClientConfigLoadingRules
	if kubeconfigPath != "" {
		loadingRules = &clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath}
	} else {
		loadingRules = clientcmd.NewDefaultClientConfigLoadingRules()
	}
	overrides := &clientcmd.ConfigOverrides{}
	if contextOverride != "" {
		overrides.CurrentContext = contextOverride
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides,
	).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("building kubeconfig: %w", err)
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating dynamic client: %w", err)
	}

	discoClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("creating discovery client: %w", err)
	}

	return &Client{
		Dynamic:   dynClient,
		Discovery: discoClient,
		Config:    config,
	}, nil
}
