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
	"fmt"

	"github.com/pkg/errors"

	"github.com/go-logr/logr"
	infrastructurev1alpha3 "github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/api/v1alpha3"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/cluster-api/controllers/external"
	"sigs.k8s.io/cluster-api/util"
	utilconversion "sigs.k8s.io/cluster-api/util/conversion"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// IBMClusterReconciler reconciles a IBMCluster object
type IBMClusterReconciler struct {
	Client     client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	restConfig *rest.Config
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmclusters/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=clusters;clusters/status,verbs=get;list;watch

func (r *IBMClusterReconciler) Reconcile(req ctrl.Request) (_ ctrl.Result, reterr error) {
	ctx := context.Background()
	log := r.Log.WithValues("ibmcluster", req.NamespacedName)

	// your logic here
	// Fetch the IBMCluster instance
	ibmCluster := &infrastructurev1alpha3.IBMCluster{}
	err := r.Client.Get(ctx, req.NamespacedName, ibmCluster)
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

	helper, err := patch.NewHelper(ibmCluster, r.Client)
	if err != nil {
		return reconcile.Result{}, errors.Wrap(err, "failed to init patch helper")
	}

	defer func() {
		e := helper.Patch(
			context.TODO(),
			ibmCluster,
		)
		if e != nil {
			fmt.Println(e.Error())
		}
	}()

	// Handle deleted clusters
	if !ibmCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ibmCluster)
	}

	return r.reconcile(ctx, ibmCluster)
}

func (r *IBMClusterReconciler) reconcile(ctx context.Context, ibmCluster *infrastructurev1alpha3.IBMCluster) (ctrl.Result, error) {
	// Call generic external reconciler.
	clusterReconcileResult, err := r.reconcileExternal(ctx, ibmCluster, ibmCluster.Spec.InfrastructureRef)
	if err != nil {
		return ctrl.Result{}, err
	}
	actualCluster := clusterReconcileResult.Result
	ibmCluster.Status.Ready = true

	if err := util.UnstructuredUnmarshalField(actualCluster, &ibmCluster.Spec.ControlPlaneEndpoint, "spec", "controlPlaneEndpoint"); err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to retrieve Spec.ControlPlaneEndpoint from infrastructure provider for Cluster %q in namespace %q",
			ibmCluster.Name, ibmCluster.Namespace)
	}

	fmt.Println(ibmCluster.Spec.ControlPlaneEndpoint)

	if err := util.UnstructuredUnmarshalField(actualCluster, &ibmCluster.Status.APIEndpoint, "status", "APIEndpoint"); err != nil && err != util.ErrUnstructuredFieldNotFound {
		return ctrl.Result{}, errors.Wrapf(err, "failed to retrieve Status.APIEndpoint from infrastructure provider for Cluster %q in namespace %q",
			ibmCluster.Name, ibmCluster.Namespace)
	}

	_ = r.Client.Update(ctx, ibmCluster)
	return ctrl.Result{}, nil
}

func (r *IBMClusterReconciler) reconcileDelete(ibmCluster *infrastructurev1alpha3.IBMCluster) (ctrl.Result, error) {
	return ctrl.Result{}, nil
}

func (r *IBMClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.restConfig = mgr.GetConfig()
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha3.IBMCluster{}).
		Complete(r)
}

func (r *IBMClusterReconciler) reconcileExternal(ctx context.Context, cluster *infrastructurev1alpha3.IBMCluster, ref *corev1.ObjectReference) (external.ReconcileOutput, error) {
	logger := r.Log.WithValues("ibmcluster", cluster.Name)

	if err := utilconversion.ConvertReferenceAPIContract(ctx, logger, r.Client, r.restConfig, ref); err != nil {
		return external.ReconcileOutput{}, err
	}

	obj, err := external.Get(ctx, r.Client, ref, cluster.Namespace)
	if err != nil {
		return external.ReconcileOutput{}, err
	}

	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return external.ReconcileOutput{}, err
	}

	// Set external object ControllerReference to the Cluster.
	if err := controllerutil.SetControllerReference(cluster, obj, r.Scheme); err != nil {
		return external.ReconcileOutput{}, err
	}

	if err := patchHelper.Patch(ctx, obj); err != nil {
		return external.ReconcileOutput{}, err
	}
	return external.ReconcileOutput{Result: obj}, nil
}
