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

// GetPod returns the pod, using the default namespace when omitted.
func (m *Manager) GetPod(ctx context.Context, namespace, name string) (*corev1.Pod, error) {
	if namespace == "" {
		namespace = corev1.NamespaceDefault
	}
	return m.client.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
}

func (m *Manager) GetPodList(ctx context.Context, namespace, label string) ([]corev1.Pod, error) {
	info, err := m.client.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: label})
	if err != nil {
		return nil, err
	}
	return info.Items, nil
}

// DeletePod requests graceful deletion. An absent pod is already deleted.
func (m *Manager) DeletePod(ctx context.Context, pod *corev1.Pod) error {
	namespace := pod.Namespace
	if namespace == "" {
		namespace = corev1.NamespaceDefault
	}
	err := m.client.CoreV1().Pods(namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{
		Preconditions: &metav1.Preconditions{UID: &pod.UID, ResourceVersion: &pod.ResourceVersion},
	})
	if apierrors.IsNotFound(err) {
		return nil
	}
	return err
}
