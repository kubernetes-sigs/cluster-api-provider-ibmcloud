/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package roks

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/api/roks/v1beta2"
	infrastructurev1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/roks/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/containerservice"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	v1beta2conditions "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/conditions/v1beta2" //nolint:staticcheck
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
)

const (
	// ClusterFinalizer allows cleanup before deletion
	ClusterFinalizer = "rokscontrolplane.infrastructure.cluster.x-k8s.io"
)

// ROKSControlPlaneReconciler reconciles a ROKSControlPlane object
type ROKSControlPlaneReconciler struct {
	client.Client
	Scheme   *runtime.Scheme
	Recorder record.EventRecorder
	APIKey   string
	Region   string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=rokscontrolplanes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=rokscontrolplanes/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=rokscontrolplanes/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the ROKSControlPlane object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.23.1/pkg/reconcile
func (r *ROKSControlPlaneReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the ROKSControlPlane instance
	roksCluster := &v1beta2.ROKSControlPlane{}
	if err := r.Get(ctx, req.NamespacedName, roksCluster); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster
	cluster, err := util.GetOwnerCluster(ctx, r.Client, roksCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Waiting for Cluster Controller to set OwnerRef on ROKSControlPlane")
		return ctrl.Result{}, nil
	}

	// Return early if the object or Cluster is paused
	if annotations.IsPaused(cluster, roksCluster) {
		log.Info("ROKSControlPlane or linked Cluster is marked as paused, not reconciling")
		return ctrl.Result{}, nil
	}

	// Create the cluster scope
	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:           r.Client,
		Cluster:          cluster,
		ROKSControlPlane: roksCluster,
		ControllerName:   "rokscontrolplane",
		APIKey:           r.APIKey,
		Region:           r.Region,
	})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create cluster scope: %w", err)
	}

	// Always close the scope when exiting this function
	defer func() {
		if err := clusterScope.Close(ctx); err != nil {
			log.Error(err, "Failed to patch ROKSControlPlane")
		}
	}()

	// Handle deletion
	if !roksCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}

	// Handle normal reconciliation
	return r.reconcileNormal(ctx, clusterScope)
}

func (r *ROKSControlPlaneReconciler) reconcileNormal(ctx context.Context, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Reconciling ROKSControlPlane")

	// Add finalizer if not present
	if !controllerutil.ContainsFinalizer(clusterScope.ROKSControlPlane, ClusterFinalizer) {
		controllerutil.AddFinalizer(clusterScope.ROKSControlPlane, ClusterFinalizer)
		// Don't return here - continue processing to avoid race condition
	}

	// Check if cluster already exists
	if clusterScope.ClusterID() == "" {
		return r.reconcileCreate(ctx, clusterScope)
	}

	// Reconcile existing cluster
	return r.reconcileUpdate(ctx, clusterScope)
}

func (r *ROKSControlPlaneReconciler) reconcileCreate(ctx context.Context, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Creating ROKS cluster")

	// Build create request
	createReq := r.buildCreateRequest(clusterScope)

	// Create the cluster
	cluster, err := clusterScope.ContainerServiceClient.CreateCluster(
		ctx,
		createReq,
		clusterScope.ResourceGroupID(),
	)
	if err != nil {
		clusterScope.Error(err, "Failed to create cluster")
		r.Recorder.Eventf(clusterScope.ROKSControlPlane, corev1.EventTypeWarning, "FailedCreate", "Failed to create cluster: %v", err)
		clusterScope.SetFailureMessage(err)
		clusterScope.SetFailureReason("CreateFailed")
		// Ensure ready is set to false
		clusterScope.ROKSControlPlane.Status.Ready = false
		return ctrl.Result{}, err
	}

	// Update status with cluster ID
	if cluster != nil {
		if cluster.ID != "" {
			clusterScope.SetClusterID(cluster.ID)
			clusterScope.Info("Cluster creation initiated", "clusterID", cluster.ID)
			r.Recorder.Eventf(clusterScope.ROKSControlPlane, corev1.EventTypeNormal, "ClusterCreated", "Cluster %s created", cluster.ID)
		}
		if cluster.State != "" {
			clusterScope.ROKSControlPlane.Status.State = cluster.State
		}
		if cluster.MasterStatus != "" {
			clusterScope.ROKSControlPlane.Status.MasterStatus = cluster.MasterStatus
		}
		clusterScope.ROKSControlPlane.Status.Ready = false // Cluster is not ready yet
	} else {
		clusterScope.Error(fmt.Errorf("cluster response is nil"), "CreateCluster returned nil cluster")
		return ctrl.Result{}, fmt.Errorf("CreateCluster returned nil cluster")
	}

	// Requeue to check status
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *ROKSControlPlaneReconciler) reconcileUpdate(ctx context.Context, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	clusterScope.Info("Updating ROKS cluster status")

	// Get cluster status from IBM Cloud
	cluster, err := clusterScope.ContainerServiceClient.GetCluster(
		ctx,
		clusterScope.ClusterID(),
		clusterScope.ResourceGroupID(),
	)
	if err != nil {
		clusterScope.Error(err, "Failed to get cluster")
		clusterScope.ROKSControlPlane.Status.Ready = false
		return ctrl.Result{}, err
	}

	// Update status
	clusterScope.ROKSControlPlane.Status.State = cluster.State
	clusterScope.ROKSControlPlane.Status.MasterStatus = cluster.MasterStatus
	clusterScope.ROKSControlPlane.Status.MasterURL = cluster.MasterURL
	clusterScope.ROKSControlPlane.Status.IngressHostname = cluster.IngressHostname
	clusterScope.ROKSControlPlane.Status.IngressSecretName = cluster.IngressSecretName
	clusterScope.ROKSControlPlane.Status.WorkerCount = cluster.WorkerCount

	// Check if cluster is ready
	// IBM Cloud ROKS clusters report "All Workers Normal" when ready, not just "Ready"
	isReady := cluster.State == "normal" &&
		(cluster.MasterStatus == "Ready" ||
			cluster.MasterStatus == "All Workers Normal" ||
			cluster.MasterStatus == "deployed")

	if isReady {
		if !clusterScope.IsReady() {
			clusterScope.Info("Cluster is ready")
			r.Recorder.Event(clusterScope.ROKSControlPlane, corev1.EventTypeNormal, "ClusterReady", "Cluster is ready")
		}

		clusterScope.SetReady()
		clusterScope.SetControlPlaneEndpoint(cluster.MasterURL)

		// Set ready condition
		v1beta2conditions.Set(clusterScope.ROKSControlPlane, metav1.Condition{
			Type:   clusterv1.ReadyCondition,
			Status: metav1.ConditionTrue,
			Reason: "ClusterReady",
		})

		// Cluster is ready, check less frequently
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// Cluster not ready yet, check more frequently
	clusterScope.SetNotReady()
	clusterScope.Info("Cluster not ready yet", "state", cluster.State, "masterStatus", cluster.MasterStatus)

	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *ROKSControlPlaneReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	if clusterScope == nil {
		return ctrl.Result{}, fmt.Errorf("clusterScope is nil in reconcileDelete")
	}

	clusterScope.Info("Deleting ROKS cluster")

	// Remove finalizer if cluster was never created
	if clusterScope.ClusterID() == "" {
		if clusterScope.ROKSControlPlane != nil {
			controllerutil.RemoveFinalizer(clusterScope.ROKSControlPlane, ClusterFinalizer)
		}
		return ctrl.Result{}, nil
	}

	// Delete the cluster
	err := clusterScope.ContainerServiceClient.DeleteCluster(
		ctx,
		clusterScope.ClusterID(),
		clusterScope.ResourceGroupID(),
	)
	if err != nil {
		clusterScope.Error(err, "Failed to delete cluster")
		if clusterScope.ROKSControlPlane != nil {
			r.Recorder.Eventf(clusterScope.ROKSControlPlane, corev1.EventTypeWarning, "FailedDelete", "Failed to delete cluster: %v", err)
			clusterScope.ROKSControlPlane.Status.Ready = false
		}
		return ctrl.Result{}, err
	}

	clusterScope.Info("Cluster deleted")
	if clusterScope.ROKSControlPlane != nil {
		r.Recorder.Event(clusterScope.ROKSControlPlane, corev1.EventTypeNormal, "ClusterDeleted", "Cluster deleted")
		// Remove finalizer
		controllerutil.RemoveFinalizer(clusterScope.ROKSControlPlane, ClusterFinalizer)
	}

	return ctrl.Result{}, nil
}

func (r *ROKSControlPlaneReconciler) buildCreateRequest(clusterScope *scope.ClusterScope) *containerservice.CreateClusterRequest {
	spec := clusterScope.ROKSControlPlane.Spec

	// Convert zones from spec to API format
	var zones []containerservice.ClusterZone
	for _, z := range spec.DefaultWorkerPool.Zones {
		zones = append(zones, containerservice.ClusterZone{
			ID:       z.ID,
			SubnetID: z.SubnetID,
		})
	}

	req := &containerservice.CreateClusterRequest{
		Name:                         spec.Name,
		MasterVersion:                spec.OpenshiftVersion,
		MachineType:                  spec.DefaultWorkerPool.Flavor,
		WorkerNum:                    spec.DefaultWorkerPool.WorkerCount,
		PodSubnet:                    spec.Network.PodSubnet,
		ServiceSubnet:                spec.Network.ServiceSubnet,
		PrivateServiceEndpoint:       spec.Network.PrivateServiceEndpoint,
		PublicServiceEndpoint:        spec.Network.PublicServiceEndpoint,
		DiskEncryption:               spec.Security.DiskEncryption,
		CosInstanceCRN:               spec.Openshift.CosInstanceCRN,
		DefaultWorkerPoolName:        spec.DefaultWorkerPool.Name,
		DefaultWorkerPoolEntitlement: spec.Openshift.Entitlement,
		// VPC-specific fields
		VPCID: spec.VPC.VPCID,
		Zones: zones,
	}

	return req
}

// SetupWithManager sets up the controller with the Manager.
func (r *ROKSControlPlaneReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta2.ROKSControlPlane{}).
		Watches(
			&clusterv1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.ClusterToROKSControlPlane),
		).
		Named("rokscontrolplane").
		Complete(r)
}

// ClusterToROKSControlPlane is a handler.ToRequestsFunc to be used to enqueue requests for reconciliation
// of ROKSControlPlane when a Cluster is updated.
func (r *ROKSControlPlaneReconciler) ClusterToROKSControlPlane(ctx context.Context, o client.Object) []ctrl.Request {
	cluster, ok := o.(*clusterv1.Cluster)
	if !ok {
		return nil
	}

	if cluster.Spec.ControlPlaneRef.Kind != "ROKSControlPlane" {
		return nil
	}

	return []ctrl.Request{
		{
			NamespacedName: client.ObjectKey{
				Namespace: cluster.Namespace,
				Name:      cluster.Spec.ControlPlaneRef.Name,
			},
		},
	}
}
