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
	"os"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/go-logr/logr"
	infrastructurev1alpha4 "github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/api/v1alpha4"
	"github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/cloud/scope"
	"github.com/pkg/errors"
	"github.com/prometheus/common/log"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/pointer"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1alpha4"
	"sigs.k8s.io/cluster-api/util"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// IBMVPCMachineReconciler reconciles a IBMVPCMachine object
type IBMVPCMachineReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcmachines/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=cluster.x-k8s.io,resources=machines;machines/status,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets;,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=get;list;watch;create;update;patch

func (r *IBMVPCMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := r.Log.WithValues("ibmvpcmachine", req.NamespacedName)

	// your logic here
	// Fetch the IBM VPC instance.

	ibmVpcMachine := &infrastructurev1alpha4.IBMVPCMachine{}
	err := r.Get(ctx, req.NamespacedName, ibmVpcMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, ibmVpcMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, ibmVpcMachine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	ibmCluster := &infrastructurev1alpha4.IBMVPCCluster{}
	ibmVpcClusterName := client.ObjectKey{
		Namespace: ibmVpcMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, ibmVpcClusterName, ibmCluster); err != nil {
		log.Info("IBMVPCCluster is not available yet")
		return ctrl.Result{}, nil
	}

	// Create the cluster scope
	iamEndpoint := os.Getenv("IAM_ENDPOINT")
	apiKey := os.Getenv("API_KEY")
	svcEndpoint := os.Getenv("SERVICE_ENDPOINT")

	// Create the machine scope
	machineScope, err := scope.NewMachineScope(scope.MachineScopeParams{
		Client:        r.Client,
		Logger:        log,
		Cluster:       cluster,
		IBMVPCCluster: ibmCluster,
		Machine:       machine,
		IBMVPCMachine: ibmVpcMachine,
	}, iamEndpoint, apiKey, svcEndpoint)
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}
	// Always close the scope when exiting this function so we can persist any IBM VPC instance changes.

	defer func() {
		if err := machineScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines
	if !ibmVpcMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machineScope)
	}

	// Handle non-deleted machines
	return r.reconcileNormal(ctx, machineScope)
}

func (r *IBMVPCMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1alpha4.IBMVPCMachine{}).
		Complete(r)
}

func (r *IBMVPCMachineReconciler) reconcileNormal(ctx context.Context, machineScope *scope.MachineScope) (ctrl.Result, error) {
	controllerutil.AddFinalizer(machineScope.IBMVPCMachine, infrastructurev1alpha4.MachineFinalizer)

	// Make sure bootstrap data is available and populated.
	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		log.Info("Bootstrap data secret reference is not yet available")
		return ctrl.Result{}, nil
	}

	if machineScope.IBMVPCCluster.Status.Subnet.ID != nil {
		machineScope.IBMVPCMachine.Spec.PrimaryNetworkInterface = infrastructurev1alpha4.NetworkInterface{
			Subnet: *machineScope.IBMVPCCluster.Status.Subnet.ID,
		}
	}

	instance, err := r.getOrCreate(machineScope)
	if err != nil {
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile VSI for IBMVPCMachine %s/%s", machineScope.IBMVPCMachine.Namespace, machineScope.IBMVPCMachine.Name)
	}

	if instance != nil {
		machineScope.IBMVPCMachine.Status.InstanceID = *instance.ID
		machineScope.IBMVPCMachine.Status.Addresses = []v1.NodeAddress{
			v1.NodeAddress{
				Type:    v1.NodeInternalIP,
				Address: *instance.PrimaryNetworkInterface.PrimaryIpv4Address,
			},
		}
		_, ok := machineScope.IBMVPCMachine.Labels[clusterv1.MachineControlPlaneLabelName]
		machineScope.IBMVPCMachine.Spec.ProviderID = pointer.StringPtr(fmt.Sprintf("ibmvpc://%s/%s", machineScope.Machine.Spec.ClusterName, machineScope.IBMVPCMachine.Name))
		if ok {
			options := &vpcv1.AddInstanceNetworkInterfaceFloatingIPOptions{}
			options.SetID(*machineScope.IBMVPCCluster.Status.APIEndpoint.FIPID)
			options.SetInstanceID(*instance.ID)
			options.SetNetworkInterfaceID(*instance.PrimaryNetworkInterface.ID)
			floatingIP, _, err :=
				machineScope.IBMVPCClients.VPCService.AddInstanceNetworkInterfaceFloatingIP(options)
			if err != nil {
				return ctrl.Result{}, errors.Wrapf(err, "failed to bind floating IP to control plane %s/%s", machineScope.IBMVPCMachine.Namespace, machineScope.IBMVPCMachine.Name)
			}
			machineScope.IBMVPCMachine.Status.Addresses = append(machineScope.IBMVPCMachine.Status.Addresses, v1.NodeAddress{
				Type:    v1.NodeExternalIP,
				Address: *floatingIP.Address,
			})
		}
		machineScope.IBMVPCMachine.Status.Ready = true
		log.Info(*instance.ID)
	}

	return ctrl.Result{}, nil
}

func (r *IBMVPCMachineReconciler) getOrCreate(scope *scope.MachineScope) (*vpcv1.Instance, error) {
	instance, err := scope.CreateMachine()
	return instance, err
}

func (r *IBMVPCMachineReconciler) reconcileDelete(scope *scope.MachineScope) (_ ctrl.Result, reterr error) {
	scope.Info("Handling deleted IBMVPCMachine")

	if err := scope.DeleteMachine(); err != nil {
		log.Info("error deleting IBMVPCMachine")
		return ctrl.Result{}, errors.Wrapf(err, "error deleting IBMVPCMachine %s/%s", scope.IBMVPCMachine.Namespace, scope.IBMVPCMachine.Spec.Name)
	}

	defer func() {
		if reterr == nil {
			// VSI is deleted so remove the finalizer.
			controllerutil.RemoveFinalizer(scope.IBMVPCMachine, infrastructurev1alpha4.MachineFinalizer)
		}
	}()

	return ctrl.Result{}, nil
}
