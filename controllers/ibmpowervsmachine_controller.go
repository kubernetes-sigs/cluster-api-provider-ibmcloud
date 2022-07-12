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
	"time"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capierrors "sigs.k8s.io/cluster-api/errors"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	infrav1beta1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	capibmrecord "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/record"
)

// IBMPowerVSMachineReconciler reconciles a IBMPowerVSMachine object.
type IBMPowerVSMachineReconciler struct {
	client.Client
	Log      logr.Logger
	Recorder record.EventRecorder
	Scheme   *runtime.Scheme
}

// dhcpCacheStore is a cache store to hold the Power VS VM DHCP IP.
var dhcpCacheStore cache.Store

func init() {
	dhcpCacheStore = powervs.InitialiseDHCPCacheStore()
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsmachines,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsmachines/status,verbs=get;update;patch

// Reconcile implements controller runtime Reconciler interface and handles reconcileation logic for IBMPowerVSMachine.
func (r *IBMPowerVSMachineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := r.Log.WithValues("ibmpowervsmachine", req.NamespacedName)

	ibmPowerVSMachine := &infrav1beta1.IBMPowerVSMachine{}
	err := r.Get(ctx, req.NamespacedName, ibmPowerVSMachine)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// Fetch the Machine.
	machine, err := util.GetOwnerMachine(ctx, r.Client, ibmPowerVSMachine.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, err
	}
	if machine == nil {
		log.Info("Machine Controller has not yet set OwnerRef")
		return ctrl.Result{}, nil
	}

	// Fetch the Cluster.
	cluster, err := util.GetClusterFromMetadata(ctx, r.Client, ibmPowerVSMachine.ObjectMeta)
	if err != nil {
		log.Info("Machine is missing cluster label or cluster does not exist")
		return ctrl.Result{}, nil
	}

	log = log.WithValues("cluster", cluster.Name)

	ibmCluster := &infrav1beta1.IBMPowerVSCluster{}
	ibmPowerVSClusterName := client.ObjectKey{
		Namespace: ibmPowerVSMachine.Namespace,
		Name:      cluster.Spec.InfrastructureRef.Name,
	}
	if err := r.Client.Get(ctx, ibmPowerVSClusterName, ibmCluster); err != nil {
		log.Info("IBMPowerVSCluster is not available yet")
		return ctrl.Result{}, nil
	}

	var ibmPowerVSImage *infrav1beta1.IBMPowerVSImage
	if ibmPowerVSMachine.Spec.ImageRef != nil {
		ibmPowerVSImage = &infrav1beta1.IBMPowerVSImage{}
		ibmPowerVSImageName := client.ObjectKey{
			Namespace: ibmPowerVSMachine.Namespace,
			Name:      ibmPowerVSMachine.Spec.ImageRef.Name,
		}
		if err := r.Client.Get(ctx, ibmPowerVSImageName, ibmPowerVSImage); err != nil {
			log.Info("IBMPowerVSImage is not available yet")
			return ctrl.Result{}, nil
		}
	}

	// Create the machine scope.
	machineScope, err := scope.NewPowerVSMachineScope(scope.PowerVSMachineScopeParams{
		Client:            r.Client,
		Logger:            log,
		Cluster:           cluster,
		IBMPowerVSCluster: ibmCluster,
		Machine:           machine,
		IBMPowerVSMachine: ibmPowerVSMachine,
		IBMPowerVSImage:   ibmPowerVSImage,
		DHCPIPCacheStore:  dhcpCacheStore,
	})
	if err != nil {
		return ctrl.Result{}, errors.Errorf("failed to create scope: %+v", err)
	}
	// Always close the scope when exiting this function so we can persist any GCPMachine changes.

	defer func() {
		if err := machineScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted machines.
	if !ibmPowerVSMachine.ObjectMeta.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(machineScope)
	}

	// Handle non-deleted machines.
	return r.reconcileNormal(machineScope)
}

// SetupWithManager creates a new IBMPowerVSMachine controller for a manager.
func (r *IBMPowerVSMachineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1beta1.IBMPowerVSMachine{}).
		Complete(r)
}

func (r *IBMPowerVSMachineReconciler) reconcileDelete(scope *scope.PowerVSMachineScope) (_ ctrl.Result, reterr error) {
	scope.Info("Handling deleted IBMPowerVSMachine")

	defer func() {
		if reterr == nil {
			// VSI is deleted so remove the finalizer.
			controllerutil.RemoveFinalizer(scope.IBMPowerVSMachine, infrav1beta1.IBMPowerVSMachineFinalizer)
		}
	}()

	if scope.IBMPowerVSMachine.Status.InstanceID == "" {
		scope.Info("InstanceID is not yet set, hence not invoking the powervs API to delete the instance")
		return ctrl.Result{}, nil
	}
	if err := scope.DeleteMachine(); err != nil {
		scope.Info("error deleting IBMPowerVSMachine")
		return ctrl.Result{}, errors.Wrapf(err, "error deleting IBMPowerVSMachine %s/%s", scope.IBMPowerVSMachine.Namespace, scope.IBMPowerVSMachine.Name)
	}
	// Remove the cached VM IP
	err := scope.DHCPIPCacheStore.Delete(powervs.VMip{Name: scope.IBMPowerVSMachine.Name})
	if err != nil {
		scope.Error(err, "failed to delete the VM entry from DHCP cache store", "VM", scope.IBMPowerVSMachine.Name)
	}
	return ctrl.Result{}, nil
}

func (r *IBMPowerVSMachineReconciler) getOrCreate(scope *scope.PowerVSMachineScope) (*models.PVMInstanceReference, error) {
	instance, err := scope.CreateMachine()
	return instance, err
}

func (r *IBMPowerVSMachineReconciler) reconcileNormal(machineScope *scope.PowerVSMachineScope) (ctrl.Result, error) {
	machineScope.Info("Reconciling IBMPowerVSMachine")

	if !machineScope.Cluster.Status.InfrastructureReady {
		machineScope.Info("Cluster infrastructure is not ready yet")
		conditions.MarkFalse(machineScope.IBMPowerVSMachine, infrav1beta1.InstanceReadyCondition, infrav1beta1.WaitingForClusterInfrastructureReason, capiv1beta1.ConditionSeverityInfo, "")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	if machineScope.IBMPowerVSImage != nil {
		if !machineScope.IBMPowerVSImage.Status.Ready {
			machineScope.Info("IBMPowerVSImage is not ready yet")
			conditions.MarkFalse(machineScope.IBMPowerVSMachine, infrav1beta1.InstanceReadyCondition, infrav1beta1.WaitingForIBMPowerVSImageReason, capiv1beta1.ConditionSeverityInfo, "")
			return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
		}
	}

	// Make sure bootstrap data is available and populated.
	if machineScope.Machine.Spec.Bootstrap.DataSecretName == nil {
		machineScope.Info("Bootstrap data secret reference is not yet available")
		conditions.MarkFalse(machineScope.IBMPowerVSMachine, infrav1beta1.InstanceReadyCondition, infrav1beta1.WaitingForBootstrapDataReason, capiv1beta1.ConditionSeverityInfo, "")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	controllerutil.AddFinalizer(machineScope.IBMPowerVSMachine, infrav1beta1.IBMPowerVSMachineFinalizer)

	ins, err := r.getOrCreate(machineScope)
	if err != nil {
		machineScope.Error(err, "unable to create instance")
		conditions.MarkFalse(machineScope.IBMPowerVSMachine, infrav1beta1.InstanceReadyCondition, infrav1beta1.InstanceProvisionFailedReason, capiv1beta1.ConditionSeverityError, err.Error())
		return ctrl.Result{}, errors.Wrapf(err, "failed to reconcile VSI for IBMPowerVSMachine %s/%s", machineScope.IBMPowerVSMachine.Namespace, machineScope.IBMPowerVSMachine.Name)
	}

	if ins != nil {
		instance, err := machineScope.IBMPowerVSClient.GetInstance(*ins.PvmInstanceID)
		if err != nil {
			return ctrl.Result{}, err
		}
		machineScope.SetProviderID(instance.PvmInstanceID)
		machineScope.SetInstanceID(instance.PvmInstanceID)
		machineScope.SetAddresses(instance)
		machineScope.SetHealth(instance.Health)
		machineScope.SetInstanceState(instance.Status)
		switch machineScope.GetInstanceState() {
		case infrav1beta1.PowerVSInstanceStateBUILD:
			machineScope.SetNotReady()
			conditions.MarkFalse(machineScope.IBMPowerVSMachine, infrav1beta1.InstanceReadyCondition, infrav1beta1.InstanceNotReadyReason, capiv1beta1.ConditionSeverityWarning, "")
		case infrav1beta1.PowerVSInstanceStateSHUTOFF:
			machineScope.SetNotReady()
			conditions.MarkFalse(machineScope.IBMPowerVSMachine, infrav1beta1.InstanceReadyCondition, infrav1beta1.InstanceStoppedReason, capiv1beta1.ConditionSeverityError, "")
			return ctrl.Result{}, nil
		case infrav1beta1.PowerVSInstanceStateACTIVE:
			machineScope.SetReady()
			conditions.MarkTrue(machineScope.IBMPowerVSMachine, infrav1beta1.InstanceReadyCondition)
		case infrav1beta1.PowerVSInstanceStateERROR:
			msg := ""
			if instance.Fault != nil {
				msg = instance.Fault.Details
			}
			machineScope.SetNotReady()
			machineScope.SetFailureReason(capierrors.UpdateMachineError)
			machineScope.SetFailureMessage(msg)
			conditions.MarkFalse(machineScope.IBMPowerVSMachine, infrav1beta1.InstanceReadyCondition, infrav1beta1.InstanceErroredReason, capiv1beta1.ConditionSeverityError, msg)
			capibmrecord.Warnf(machineScope.IBMPowerVSMachine, "FailedBuildInstance", "Failed to build the instance - %s", msg)
			return ctrl.Result{}, nil
		default:
			machineScope.SetNotReady()
			machineScope.Info("PowerVS instance state is undefined", "state", *instance.Status, "instance-id", machineScope.GetInstanceID())
			conditions.MarkUnknown(machineScope.IBMPowerVSMachine, infrav1beta1.InstanceReadyCondition, "", "")
		}
		machineScope.Info(*ins.PvmInstanceID)
	}

	// Requeue after 2 minute if machine is not ready to update status of the machine properly.
	if !machineScope.IsReady() {
		return ctrl.Result{RequeueAfter: 2 * time.Minute}, nil
	}

	return ctrl.Result{}, nil
}
