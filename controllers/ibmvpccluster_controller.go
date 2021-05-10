/*


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

	"github.com/go-logr/logr"
	infrastructurev1alpha3 "github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/api/v1alpha3"
	"github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/cloud/scope"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha3"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
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

func (r *IBMVPCClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.Background()
	log := r.Log.WithValues("ibmvpccluster", req.NamespacedName)

	// your logic here
	// Fetch the IBMVPCCluster instance
	ibmVPCCluster := &infrastructurev1alpha3.IBMVPCCluster{}
	err := r.Get(ctx, req.NamespacedName, ibmVPCCluster)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	ibmCluster, err := GetOwnerIBMCluster(ctx, r.Client, ibmVPCCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if ibmCluster == nil {
		log.Info("IBMVPCCluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// Fetch the Cluster.
	cluster, err := util.GetOwnerCluster(ctx, r.Client, ibmCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if cluster == nil {
		log.Info("IBMCluster Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// Create the scope.
	iamEndpoint := os.Getenv("IAM_ENDPOINT")
	apiKey := os.Getenv("API_KEY")
	svcEndpoint := os.Getenv("SERVICE_ENDPOINT")

	clusterScope, err := scope.NewClusterScope(scope.ClusterScopeParams{
		Client:        r.Client,
		Logger:        log,
		Cluster:       cluster,
		IBMCluster:    ibmCluster,
		IBMVPCCluster: ibmVPCCluster,
	}, iamEndpoint, apiKey, svcEndpoint)

	// Always close the scope when exiting this function so we can persist any IBM VPC instance changes.
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
	} else {
		return r.reconcile(ctx, clusterScope)
	}
}

func (r *IBMVPCClusterReconciler) reconcile(ctx context.Context, clusterScope *scope.ClusterScope) (ctrl.Result, error) {
	if !controllerutil.ContainsFinalizer(clusterScope.IBMVPCCluster, infrastructurev1alpha3.VPCClusterFinalizer) {
		controllerutil.AddFinalizer(clusterScope.IBMVPCCluster, infrastructurev1alpha3.VPCClusterFinalizer)
		//_ = r.Update(ctx, clusterScope.IBMVPCCluster)
		return ctrl.Result{}, nil
	}

	vpc, err := clusterScope.CreateVPC()
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile VPC for IBMVPCCluster %s/%s", clusterScope.IBMVPCCluster.Namespace, clusterScope.IBMVPCCluster.Name)
	}
	if vpc != nil {
		clusterScope.IBMVPCCluster.Status.VPC = infrastructurev1alpha3.VPC{
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

			clusterScope.IBMVPCCluster.Status.APIEndpoint = infrastructurev1alpha3.APIEndpoint{
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
			clusterScope.IBMVPCCluster.Status.Subnet = infrastructurev1alpha3.Subnet{
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
	if err := clusterScope.DeleteVPC(); err != nil {
		return ctrl.Result{}, errors.Wrap(err, "failed to delete VPC")
	} else {
		controllerutil.RemoveFinalizer(clusterScope.IBMVPCCluster, infrastructurev1alpha3.ClusterFinalizer)
		return ctrl.Result{}, nil
	}
}

func (r *IBMVPCClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha3.IBMVPCCluster{}).
		Complete(r)
}

// GetOwnerCluster returns the Cluster object owning the current resource.
func GetOwnerIBMCluster(ctx context.Context, c client.Client, obj metav1.ObjectMeta) (*infrastructurev1alpha3.IBMCluster, error) {
	for _, ref := range obj.OwnerReferences {
		if ref.Kind != "IBMCluster" {
			continue
		}
		gv, err := schema.ParseGroupVersion(ref.APIVersion)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		// hard code the group information right now. IT SHOULD BE DEFINED SOMEWHERE
		if gv.Group == "infrastructure.cluster.x-k8s.io" {
			return GetIBMClusterByName(ctx, c, obj.Namespace, ref.Name)
		}
	}
	return nil, nil
}

// GetClusterByName finds and return a Cluster object using the specified params.
func GetIBMClusterByName(ctx context.Context, c client.Client, namespace, name string) (*infrastructurev1alpha3.IBMCluster, error) {
	ibmCluster := &infrastructurev1alpha3.IBMCluster{}
	key := client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}

	if err := c.Get(ctx, key, ibmCluster); err != nil {
		return nil, err
	}

	return ibmCluster, nil
}
