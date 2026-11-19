package container

import (
	"errors"
	"os"
	"strings"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Manager struct {
	client    *kubernetes.Clientset
	namespace string
}

func (m *Manager) Namespace() string {
	return m.namespace
}

func NewK8sManager() (*Manager, error) {
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return nil, err
	}
	namespace := strings.TrimSpace(string(data))
	if namespace == "" {
		return nil, errors.New("current pod namespace is empty")
	}
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &Manager{
		client:    client,
		namespace: namespace,
	}, nil
}
