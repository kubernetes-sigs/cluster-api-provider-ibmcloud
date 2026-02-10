/*
Copyright 2022 The Kubernetes Authors.

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

package controllers

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/v2/api/v1beta2"
	ekscontrolplanev1 "sigs.k8s.io/cluster-api-provider-ibmcloud/v2/controlplane/vpc/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/v2/pkg/logger"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/cluster-api/util/predicates"
)

// IBMVPCManagedClusterReconciler reconciles IBMVPCManagedCluster.
type IBMVPCManagedClusterReconciler struct {
	client.Client
	Recorder         record.EventRecorder
	WatchFilterValue string
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcmanagedclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcmanagedclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=controlplane.cluster.x-k8s.io,resources=ibmvpcmanagedcontrolplanes;ibmvpcmanagedcontrolplanes/status,verbs=get;list;watch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete

func (r *IBMVPCManagedClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the IBMVPCManagedCluster instance
	ibmvpcManagedCluster := &infrav1.IBMVPCManagedCluster{}
	err := r.Get(ctx, req.NamespacedName, ibmvpcManagedCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, ibmvpcManagedCluster.ObjectMeta)
	if err != nil {
		return reconcile.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return reconcile.Result{}, nil
	}

	if annotations.IsPaused(cluster, ibmvpcManagedCluster) {
		log.Info("IBMVPCManagedCluster or linked Cluster is marked as paused. Won't reconcile")
		return reconcile.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	controlPlane := &ekscontrolplanev1.IBMVPCManagedControlPlane{}
	controlPlaneRef := types.NamespacedName{
		Name:      cluster.Spec.ControlPlaneRef.Name,
		Namespace: cluster.Spec.ControlPlaneRef.Namespace,
	}

	if err := r.Get(ctx, controlPlaneRef, controlPlane); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to get control plane ref: %w", err)
	}

	log = log.WithValues("controlPlane", controlPlaneRef.Name)

	patchHelper, err := patch.NewHelper(ibmvpcManagedCluster, r.Client)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}

	// Set the values from the managed control plane
	ibmvpcManagedCluster.Status.Ready = true
	ibmvpcManagedCluster.Spec.ControlPlaneEndpoint = controlPlane.Spec.ControlPlaneEndpoint
	ibmvpcManagedCluster.Status.FailureDomains = controlPlane.Status.FailureDomains

	if err := patchHelper.Patch(ctx, ibmvpcManagedCluster); err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to patch IBMVPCManagedCluster: %w", err)
	}

	log.Info("Successfully reconciled IBMVPCManagedCluster")

	return reconcile.Result{}, nil
}

func (r *IBMVPCManagedClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager, options controller.Options) error {
	log := logger.FromContext(ctx)

	ibmvpcManagedCluster := &infrav1.IBMVPCManagedCluster{}

	controller, err := ctrl.NewControllerManagedBy(mgr).
		WithOptions(options).
		For(ibmvpcManagedCluster).
		WithEventFilter(predicates.ResourceNotPausedAndHasFilterLabel(ctrl.LoggerFrom(ctx), r.WatchFilterValue)).
		Build(r)

	if err != nil {
		return fmt.Errorf("error creating controller: %w", err)
	}

	// Add a watch for clusterv1.Cluster unpaise
	if err = controller.Watch(
		&source.Kind{Type: &clusterv1.Cluster{}},
		handler.EnqueueRequestsFromMapFunc(util.ClusterToInfrastructureMapFunc(ctx, infrav1.GroupVersion.WithKind("IBMVPCManagedCluster"), mgr.GetClient(), &infrav1.IBMVPCManagedCluster{})),
		predicates.ClusterUnpaused(log.GetLogger()),
	); err != nil {
		return fmt.Errorf("failed adding a watch for ready clusters: %w", err)
	}

	// Add a watch for IBMVPCManagedControlPlane
	if err = controller.Watch(
		&source.Kind{Type: &ekscontrolplanev1.IBMVPCManagedControlPlane{}},
		handler.EnqueueRequestsFromMapFunc(r.managedControlPlaneToManagedCluster(ctx, log)),
	); err != nil {
		return fmt.Errorf("failed adding watch on IBMVPCManagedControlPlane: %w", err)
	}

	return nil
}

func (r *IBMVPCManagedClusterReconciler) managedControlPlaneToManagedCluster(ctx context.Context, log *logger.Logger) handler.MapFunc {
	return func(o client.Object) []ctrl.Request {
		ibmvpcManagedControlPlane, ok := o.(*ekscontrolplanev1.IBMVPCManagedControlPlane)
		if !ok {
			log.Error(errors.Errorf("expected an IBMVPCManagedControlPlane, got %T instead", o), "failed to map IBMVPCManagedControlPlane")
			return nil
		}

		log := log.WithValues("objectMapper", "ibmvpcmcpTomc", "ibmvpcmanagedcontrolplane", klog.KRef(ibmvpcManagedControlPlane.Namespace, ibmvpcManagedControlPlane.Name))

		if !ibmvpcManagedControlPlane.ObjectMeta.DeletionTimestamp.IsZero() {
			log.Info("IBMVPCManagedControlPlane has a deletion timestamp, skipping mapping")
			return nil
		}

		if ibmvpcManagedControlPlane.Spec.ControlPlaneEndpoint.IsZero() {
			log.Debug("IBMVPCManagedControlPlane has no control plane endpoint, skipping mapping")
			return nil
		}

		cluster, err := util.GetOwnerCluster(ctx, r.Client, ibmvpcManagedControlPlane.ObjectMeta)
		if err != nil {
			log.Error(err, "failed to get owning cluster")
			return nil
		}
		if cluster == nil {
			log.Info("no owning cluster, skipping mapping")
			return nil
		}

		managedClusterRef := cluster.Spec.InfrastructureRef
		if managedClusterRef == nil || managedClusterRef.Kind != "IBMVPCManagedCluster" {
			log.Info("InfrastructureRef is nil or not IBMVPCManagedCluster, skipping mapping")
			return nil
		}

		return []ctrl.Request{
			{
				NamespacedName: types.NamespacedName{
					Name:      managedClusterRef.Name,
					Namespace: managedClusterRef.Namespace,
				},
			},
		}
	}
}
