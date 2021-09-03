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

	"github.com/go-logr/logr"
	infrastructurev1alpha4 "github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/api/v1alpha4"
	v1 "k8s.io/api/core/v1"
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
)

// IBMMachineReconciler reconciles a IBMMachine object
type IBMMachineReconciler struct {
	client.Client
	Log        logr.Logger
	Scheme     *runtime.Scheme
	restConfig *rest.Config
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmmachines/status,verbs=get;update;patch

func (r *IBMMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := r.Log.WithValues("ibmmachine", req.NamespacedName)

	// your logic here
	// Fetch the IBM VPC instance.

	ibmMachine := &infrastructurev1alpha4.IBMMachine{}
	err := r.Get(ctx, req.NamespacedName, ibmMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, ibmMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("IBMMachine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, ibmMachine.ObjectMeta)
	if err != nil {
		log.Info("IBMMachine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	ibmCluster := &infrastructurev1alpha4.IBMCluster{}
	ibmClusterName := client.ObjectKey{
		Namespace: ibmMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, ibmClusterName, ibmCluster); err != nil {
		log.Info("IBMCluster is not available yet")
		return ctrl.Result{}, nil
	}

	// Handle deleted machines
	if !ibmMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ibmMachine)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, ibmCluster, ibmMachine)
}

func (r *IBMMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha4.IBMMachine{}).
		Complete(r)
}

func (r *IBMMachineReconciler) reconcileNormal(ctx context.Context, ibmCluster *infrastructurev1alpha4.IBMCluster, ibmMachine *infrastructurev1alpha4.IBMMachine) (ctrl.Result, error) {
	//controllerutil.AddFinalizer(machineScope.IBMMachine, infrastructurev1alpha4.MachineFinalizer)

	_, err := r.reconcileExternal(ctx, ibmCluster, ibmMachine, ibmMachine.Spec.InfrastructureRef)
	if err != nil {
		return ctrl.Result{}, err
	}

	ibmCluster.Status.Ready = true
	_ = r.Client.Update(ctx, ibmCluster)
	return ctrl.Result{}, nil
}

func (r *IBMMachineReconciler) reconcileDelete(ibmMachine *infrastructurev1alpha4.IBMMachine) (_ ctrl.Result, reterr error) {
	return ctrl.Result{}, nil
}

func (r *IBMMachineReconciler) reconcileExternal(ctx context.Context, cluster *infrastructurev1alpha4.IBMCluster, machine *infrastructurev1alpha4.IBMMachine, ref *v1.ObjectReference) (external.ReconcileOutput, error) {
	if err := utilconversion.ConvertReferenceAPIContract(ctx, r.Client, r.restConfig, ref); err != nil {
		return external.ReconcileOutput{}, err
	}

	obj, err := external.Get(ctx, r.Client, ref, machine.Namespace)
	if err != nil {
		return external.ReconcileOutput{}, err
	}

	patchHelper, err := patch.NewHelper(obj, r.Client)
	if err != nil {
		return external.ReconcileOutput{}, err
	}

	// Set external object ControllerReference to the Cluster.
	if err := controllerutil.SetControllerReference(machine, obj, r.Scheme); err != nil {
		return external.ReconcileOutput{}, err
	}

	if err := patchHelper.Patch(ctx, obj); err != nil {
		return external.ReconcileOutput{}, err
	}
	return external.ReconcileOutput{}, nil
}
