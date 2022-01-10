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
	"os"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/authenticator"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/IBM/vpc-go-sdk/vpcv1"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/predicates"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	infrastructurev1beta1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
)

// IBMVPCClusterReconciler reconciles a IBMVPCCluster object
type IBMVPCClusterReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

// Reconcile implements controller runtime Reconciler interface and handles reconcileation logic for IBMVPCCluster.
func (r *IBMVPCClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := r.Log.WithValues("ibmvpccluster", req.NamespacedName)

	// your logic here
	// Fetch the IBMVPCCluster instance
	ibmCluster := &infrastructurev1beta1.IBMVPCCluster{}
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
	// TODO: Will be removed once we find a better way of overriding the service endpoint, generate via spec
	svcEndpoint := os.Getenv("SERVICE_ENDPOINT")

	authenticator, err := authenticator.GetAuthenticator()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to get authenticator")
	}

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:        r.Client,
		Logger:        log,
		Cluster:       cluster,
		IBMVPCCluster: ibmCluster,
	}, authenticator, svcEndpoint)

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

func (r *IBMVPCClusterReconciler) reconcile(ctx context.Context, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(clusterScope.IBMVPCCluster, infrastructurev1beta1.ClusterFinalizer) {
		controllerutil.AddFinalizer(clusterScope.IBMVPCCluster, infrastructurev1beta1.ClusterFinalizer)
		//_ = r.Update(ctx, clusterScope.IBMVPCCluster)
		return ctrl.Result{}, nil
	}

	vpc, err := clusterScope.CreateVPC()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile VPC for IBMVPCCluster %s/%s", clusterScope.IBMVPCCluster.Namespace, clusterScope.IBMVPCCluster.Name)
	}
	if vpc != nil {
		clusterScope.IBMVPCCluster.Status.VPC = infrastructurev1beta1.VPC{
			ID:   *vpc.ID,
			Name: *vpc.Name,
		}
	}

	if clusterScope.IBMVPCCluster.Spec.ControlPlaneEndpoint.Host == "" {
		fip, err := clusterScope.ReserveFIP()
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile Control Plane Endpoint for IBMVPCCluster %s/%s", clusterScope.IBMVPCCluster.Namespace, clusterScope.IBMVPCCluster.Name)
		}

		if fip != nil {
			clusterScope.IBMVPCCluster.Spec.ControlPlaneEndpoint = clusterv1.APIEndpoint{
				Host: *fip.Address,
				Port: 6443,
			}

			clusterScope.IBMVPCCluster.Status.VPCEndpoint = infrastructurev1beta1.VPCEndpoint{
				Address: fip.Address,
				FIPID:   fip.ID,
			}
		}
	}

	if clusterScope.IBMVPCCluster.Status.Subnet.ID == nil {
		subnet, err := clusterScope.CreateSubnet()
		if err != nil {
			return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile Subnet for IBMVPCCluster %s/%s", clusterScope.IBMVPCCluster.Namespace, clusterScope.IBMVPCCluster.Name)
		}
		if subnet != nil {
			clusterScope.IBMVPCCluster.Status.Subnet = infrastructurev1beta1.Subnet{
				Ipv4CidrBlock: subnet.Ipv4CIDRBlock,
				Name:          subnet.Name,
				ID:            subnet.ID,
				Zone:          subnet.Zone.Name,
			}
		}
	}

	clusterScope.IBMVPCCluster.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *IBMVPCClusterReconciler) reconcileDelete(clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	// check if still have existing VSIs
	listVSIOpts := &vpcv1.ListInstancesOptions{
		VPCID: &clusterScope.IBMVPCCluster.Status.VPC.ID,
	}
	vsis, _, err := clusterScope.VPCService.ListInstances(listVSIOpts)
	if err != nil {
		return ctrl.Result{}, errors.Wrap(err, "Error when listing VSIs when tried to delete subnet ")
	}
	// skip deleting other resources if still have vsis running
	if *vsis.TotalCount != int64(0) {
		return ctrl.Result{}, nil
	}

	if err := clusterScope.DeleteSubnet(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete subnet")
	}

	if err := clusterScope.DeleteFloatingIP(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete floatingIP")
	}

	if err := clusterScope.DeleteVPC(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete VPC")
	}
	controllerutil.RemoveFinalizer(clusterScope.IBMVPCCluster, infrastructurev1beta1.ClusterFinalizer)
	return ctrl.Result{}, nil
}

// SetupWithManager creates a new IBMVPCCluster controller for a manager.
func (r *IBMVPCClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.IBMVPCCluster{}).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(ctrl.LoggerFrom(context.TODO()))).
		Complete(r)
}
