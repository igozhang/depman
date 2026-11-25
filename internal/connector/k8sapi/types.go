// Package k8sapi defines a portable cluster management surface.
// Concrete backends (Rancher, kubeconfig, …) implement Client.
package k8sapi

import (
	"context"

	"depman/internal/connector"
)

// ObjectRef identifies a namespaced Kubernetes-style object.
type ObjectRef struct {
	Namespace string
	Name      string
	Kind      string // Deployment | StatefulSet | DaemonSet | …
}

// ClusterInfo is a backend-agnostic cluster summary.
type ClusterInfo struct {
	ID   string
	Name string
	State string
}

// NamespaceInfo is a backend-agnostic namespace summary.
type NamespaceInfo struct {
	Name   string
	Status string
}

// WorkloadInfo is a backend-agnostic workload summary.
type WorkloadInfo struct {
	Ref      ObjectRef
	Replicas int32
	Ready    int32
	Image    string
}

// ScaleSpec requests a replica change.
type ScaleSpec struct {
	Ref      ObjectRef
	Replicas int32
}

// Client is the portable cluster API (read + write).
type Client interface {
	connector.Pinger

	ListClusters(ctx context.Context) ([]ClusterInfo, error)
	ListNamespaces(ctx context.Context, clusterID string) ([]NamespaceInfo, error)
	ListWorkloads(ctx context.Context, clusterID, namespace string) ([]WorkloadInfo, error)

	Scale(ctx context.Context, clusterID string, spec ScaleSpec) error
	Restart(ctx context.Context, clusterID string, ref ObjectRef) error
}
