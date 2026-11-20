package container

import (
	"context"
	"errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreatePod creates the supplied pod in its namespace, defaulting to "default".
func (m *Manager) CreatePod(ctx context.Context, pod *corev1.Pod) (*corev1.Pod, error) {
	if pod == nil {
		return nil, errors.New("pod is required")
	}
	namespace := pod.Namespace
	if namespace == "" {
		namespace = corev1.NamespaceDefault
	}
	return m.client.CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
}

// GetPodStatus returns the pod's current status, using the default namespace when omitted.
func (m *Manager) GetPodStatus(ctx context.Context, namespace, name string) (*corev1.PodStatus, error) {
	if namespace == "" {
		namespace = corev1.NamespaceDefault
	}
	pod, err := m.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return &pod.Status, nil
}

// DeletePod requests graceful deletion. An absent pod is already deleted.
func (m *Manager) DeletePod(ctx context.Context, namespace, name string) error {
	if namespace == "" {
		namespace = corev1.NamespaceDefault
	}
	err := m.client.CoreV1().Pods(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
