/*
Copyright 2021 The Kubernetes Authors.

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

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1alpha4"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
)

// IBMPowerVSClusterReconciler reconciles a IBMPowerVSCluster object
type IBMPowerVSClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsclusters/status,verbs=get;update;patch

// Reconcile implements controller runtime Reconciler interface and handles reconcileation logic for IBMPowerVSCluster.
func (r *IBMPowerVSClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := r.Log.WithValues("ibmpowervscluster", req.NamespacedName)

	// Fetch the IBMPowerVSCluster instance
	ibmCluster := &v1alpha4.IBMPowerVSCluster{}
	err := r.Get(ctx, req.NamespacedName, ibmCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, ibmCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("Cluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// Create the scope.
	clusterScope, err := scope.NewPowerVSClusterScope(scope.PowerVSClusterScopeParams{
		Client:            r.Client,
		Logger:            log,
		Cluster:           cluster,
		IBMPowerVSCluster: ibmCluster,
	})

	// Always close the scope when exiting this function so we can persist any GCPMachine changes.
	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters
	if !ibmCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(clusterScope)
	}

	if err != nil {
		return reconcile.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}
	return r.reconcile(ctx, clusterScope)
}

func (r *IBMPowerVSClusterReconciler) reconcile(ctx context.Context, clusterScope *scope.PowerVSClusterScope) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(clusterScope.IBMPowerVSCluster, v1alpha4.IBMPowerVSClusterFinalizer) {
		controllerutil.AddFinalizer(clusterScope.IBMPowerVSCluster, v1alpha4.IBMPowerVSClusterFinalizer)
		return ctrl.Result{}, nil
	}

	clusterScope.IBMPowerVSCluster.Status.Ready = true

	return ctrl.Result{}, nil
}

func (r *IBMPowerVSClusterReconciler) reconcileDelete(clusterScope *scope.PowerVSClusterScope) (ctrl.Result, error) {
	controllerutil.RemoveFinalizer(clusterScope.IBMPowerVSCluster, v1alpha4.IBMPowerVSClusterFinalizer)
	return ctrl.Result{}, nil
}

// SetupWithManager creates a new IBMPowerVSCluster controller for a manager.
func (r *IBMPowerVSClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha4.IBMPowerVSCluster{}).
		Complete(r)
}
