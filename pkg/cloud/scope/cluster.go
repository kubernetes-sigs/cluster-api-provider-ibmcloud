package scope

import (
	"context"
	"fmt"

	"k8s.io/klog/v2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/api/roks/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/containerservice"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ClusterScopeParams defines the input parameters used to create a new ClusterScope
type ClusterScopeParams struct {
	Client           client.Client
	Cluster          *clusterv1.Cluster
	ROKSControlPlane *v1beta2.ROKSControlPlane
	ControllerName   string
	APIKey           string
	Region           string
}

// ClusterScope defines a scope defined around a cluster and its ROKSControlPlane
type ClusterScope struct {
	client             client.Client
	patchHelper        *patch.Helper
	clusterPatchHelper *patch.Helper

	Cluster          *clusterv1.Cluster
	ROKSControlPlane *v1beta2.ROKSControlPlane

	ContainerServiceClient *containerservice.Client
	controllerName         string
}

// NewClusterScope creates a new ClusterScope from the supplied parameters
func NewClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Client == nil {
		return nil, fmt.Errorf("client is required when creating a ClusterScope")
	}
	if params.Cluster == nil {
		return nil, fmt.Errorf("cluster is required when creating a ClusterScope")
	}
	if params.ROKSControlPlane == nil {
		return nil, fmt.Errorf("rokscontrolplane is required when creating a ClusterScope")
	}

	helper, err := patch.NewHelper(params.ROKSControlPlane, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init patch helper: %w", err)
	}

	clusterHelper, err := patch.NewHelper(params.Cluster, params.Client)
	if err != nil {
		return nil, fmt.Errorf("failed to init cluster patch helper: %w", err)
	}

	// Create container service client
	containerServiceClient, err := containerservice.NewClient(params.APIKey, params.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to create container service client: %w", err)
	}

	return &ClusterScope{
		client:                 params.Client,
		patchHelper:            helper,
		clusterPatchHelper:     clusterHelper,
		Cluster:                params.Cluster,
		ROKSControlPlane:       params.ROKSControlPlane,
		ContainerServiceClient: containerServiceClient,
		controllerName:         params.ControllerName,
	}, nil
}

// Close closes the cluster scope by updating the cluster spec and status
func (s *ClusterScope) Close(ctx context.Context) error {
	// Patch both the ROKSControlPlane and the parent Cluster
	if err := s.patchHelper.Patch(ctx, s.ROKSControlPlane); err != nil {
		return fmt.Errorf("failed to patch ROKSControlPlane: %w", err)
	}
	if err := s.clusterPatchHelper.Patch(ctx, s.Cluster); err != nil {
		return fmt.Errorf("failed to patch Cluster: %w", err)
	}
	return nil
}

// Name returns the cluster name
func (s *ClusterScope) Name() string {
	return s.Cluster.Name
}

// Namespace returns the cluster namespace
func (s *ClusterScope) Namespace() string {
	return s.Cluster.Namespace
}

// ClusterName returns the ROKSControlPlane name
func (s *ClusterScope) ClusterName() string {
	return s.ROKSControlPlane.Name
}

// ResourceGroupID returns the resource group ID
func (s *ClusterScope) ResourceGroupID() string {
	return s.ROKSControlPlane.Spec.ResourceGroupID
}

// SetReady sets the cluster ready status
func (s *ClusterScope) SetReady() {
	s.ROKSControlPlane.Status.Ready = true
}

// SetNotReady sets the cluster not ready status
func (s *ClusterScope) SetNotReady() {
	s.ROKSControlPlane.Status.Ready = false
}

// IsReady returns true if the cluster is ready
func (s *ClusterScope) IsReady() bool {
	return s.ROKSControlPlane.Status.Ready
}

// SetFailureMessage sets the cluster failure message
func (s *ClusterScope) SetFailureMessage(err error) {
	msg := err.Error()
	s.ROKSControlPlane.Status.FailureMessage = &msg
}

// SetFailureReason sets the cluster failure reason
func (s *ClusterScope) SetFailureReason(reason string) {
	s.ROKSControlPlane.Status.FailureReason = &reason
}

// SetControlPlaneEndpoint sets the control plane endpoint
func (s *ClusterScope) SetControlPlaneEndpoint(endpoint string) {
	// Set on both the ROKSControlPlane and the parent Cluster
	s.ROKSControlPlane.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: endpoint,
		Port: 443,
	}
	s.Cluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
		Host: endpoint,
		Port: 443,
	}
}

// ControlPlaneEndpoint returns the control plane endpoint
func (s *ClusterScope) ControlPlaneEndpoint() clusterv1.APIEndpoint {
	return s.ROKSControlPlane.Spec.ControlPlaneEndpoint
}

// ClusterID returns the IBM Cloud cluster ID
func (s *ClusterScope) ClusterID() string {
	return s.ROKSControlPlane.Status.ClusterID
}

// SetClusterID sets the IBM Cloud cluster ID
func (s *ClusterScope) SetClusterID(id string) {
	s.ROKSControlPlane.Status.ClusterID = id
}

// Info logs an info message
func (s *ClusterScope) Info(msg string, keysAndValues ...interface{}) {
	klog.InfoS(msg, append([]interface{}{
		"cluster", s.ClusterName(),
		"namespace", s.Namespace(),
	}, keysAndValues...)...)
}

// Error logs an error message
func (s *ClusterScope) Error(err error, msg string, keysAndValues ...interface{}) {
	klog.ErrorS(err, msg, append([]interface{}{
		"cluster", s.ClusterName(),
		"namespace", s.Namespace(),
	}, keysAndValues...)...)
}
