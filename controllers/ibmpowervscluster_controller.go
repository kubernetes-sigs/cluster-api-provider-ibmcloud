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
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/tools/record"
	"k8s.io/klog/v2"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/predicates"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/endpoints"
)

// IBMPowerVSClusterReconciler reconciles a IBMPowerVSCluster object.
type IBMPowerVSClusterReconciler struct {
	client.Client
	Recorder        record.EventRecorder
	ServiceEndpoint []endpoints.ServiceEndpoint
	Scheme          *runtime.Scheme

	ClientFactory scope.ClientFactory
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsclusters,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsclusters/status,verbs=get;update;patch

// Reconcile implements controller runtime Reconciler interface and handles reconcileation logic for IBMPowerVSCluster.
func (r *IBMPowerVSClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	// Fetch the IBMPowerVSCluster instance.
	ibmCluster := &infrav1beta2.IBMPowerVSCluster{}
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
	log = log.WithValues("cluster", klog.KObj(cluster))

	// Create the scope.
	clusterScope, err := scope.NewPowerVSClusterScope(scope.PowerVSClusterScopeParams{
		Client:            r.Client,
		Logger:            log,
		Cluster:           cluster,
		IBMPowerVSCluster: ibmCluster,
		ServiceEndpoint:   r.ServiceEndpoint,
		ClientFactory:     r.ClientFactory,
	})

	if err != nil {
		return reconcile.Result{}, fmt.Errorf("failed to create scope: %w", err)
	}

	// Always close the scope when exiting this function so we can persist any IBMPowerVSCluster changes.
	defer func() {
		if err := clusterScope.Close(); err != nil && reterr == nil {
			reterr = err
		}
	}()

	// Handle deleted clusters.
	if !ibmCluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, clusterScope)
	}

	return r.reconcile(clusterScope)
}

type powerVSCluster struct {
	cluster *infrav1beta2.IBMPowerVSCluster
	mu      sync.Mutex
}

type reconcileResult struct {
	reconcile.Result
	error
}

func (update *powerVSCluster) updateCondition(condition bool, conditionArgs ...interface{}) {
	update.mu.Lock()
	defer update.mu.Unlock()
	if condition {
		conditions.MarkTrue(update.cluster, conditionArgs[0].(capiv1beta1.ConditionType))
		return
	}

	conditions.MarkFalse(update.cluster, conditionArgs[0].(capiv1beta1.ConditionType), conditionArgs[1].(string), conditionArgs[2].(capiv1beta1.ConditionSeverity), conditionArgs[3].(string), conditionArgs[4:]...)
}

func (r *IBMPowerVSClusterReconciler) reconcilePowerVSResources(clusterScope *scope.PowerVSClusterScope, powerVSCluster *powerVSCluster, ch chan reconcileResult, wg *sync.WaitGroup) {
	defer wg.Done()
	powerVSLog := clusterScope.WithName("powervs")
	// reconcile PowerVS service instance
	powerVSLog.Info("Reconciling PowerVS service instance")
	if requeue, err := clusterScope.ReconcilePowerVSServiceInstance(); err != nil {
		powerVSLog.Error(err, "failed to reconcile PowerVS service instance")
		powerVSCluster.updateCondition(false, infrav1beta2.ServiceInstanceReadyCondition, infrav1beta2.ServiceInstanceReconciliationFailedReason, capiv1beta1.ConditionSeverityError, err.Error())
		ch <- reconcileResult{reconcile.Result{}, err}
		return
	} else if requeue {
		powerVSLog.Info("PowerVS service instance creation is pending, requeuing")
		ch <- reconcileResult{reconcile.Result{Requeue: true}, nil}
		return
	}
	powerVSCluster.updateCondition(true, infrav1beta2.ServiceInstanceReadyCondition)

	clusterScope.IBMPowerVSClient.WithClients(powervs.ServiceOptions{CloudInstanceID: clusterScope.GetServiceInstanceID()})

	// reconcile network
	powerVSLog.Info("Reconciling network")
	if dhcpServerActive, err := clusterScope.ReconcileNetwork(); err != nil {
		powerVSLog.Error(err, "failed to reconcile PowerVS network")
		powerVSCluster.updateCondition(false, infrav1beta2.NetworkReadyCondition, infrav1beta2.NetworkReconciliationFailedReason, capiv1beta1.ConditionSeverityError, err.Error())
		ch <- reconcileResult{reconcile.Result{}, err}
		return
	} else if dhcpServerActive {
		powerVSCluster.updateCondition(true, infrav1beta2.NetworkReadyCondition)
		return
	}
	// Do not want to block the reconciliation of other resources like setting up TG and COS, so skipping the requeue and only logging the info.
	powerVSLog.Info("PowerVS network creation is pending")
}

func (r *IBMPowerVSClusterReconciler) reconcileVPCResources(clusterScope *scope.PowerVSClusterScope, powerVSCluster *powerVSCluster, ch chan reconcileResult, wg *sync.WaitGroup) {
	defer wg.Done()
	vpcLog := clusterScope.WithName("vpc")
	vpcLog.Info("Reconciling VPC")
	if requeue, err := clusterScope.ReconcileVPC(); err != nil {
		clusterScope.Error(err, "failed to reconcile VPC")
		powerVSCluster.updateCondition(false, infrav1beta2.VPCReadyCondition, infrav1beta2.VPCReconciliationFailedReason, capiv1beta1.ConditionSeverityError, err.Error())
		ch <- reconcileResult{reconcile.Result{}, err}
		return
	} else if requeue {
		vpcLog.Info("VPC creation is pending, requeuing")
		ch <- reconcileResult{reconcile.Result{Requeue: true}, nil}
		return
	}
	powerVSCluster.updateCondition(true, infrav1beta2.VPCReadyCondition)

	// reconcile VPC Subnet
	vpcLog.Info("Reconciling VPC subnets")
	if requeue, err := clusterScope.ReconcileVPCSubnets(); err != nil {
		vpcLog.Error(err, "failed to reconcile VPC subnets")
		powerVSCluster.updateCondition(false, infrav1beta2.VPCSubnetReadyCondition, infrav1beta2.VPCSubnetReconciliationFailedReason, capiv1beta1.ConditionSeverityError, err.Error())
		ch <- reconcileResult{reconcile.Result{}, err}
		return
	} else if requeue {
		vpcLog.Info("VPC subnet creation is pending, requeuing")
		ch <- reconcileResult{reconcile.Result{Requeue: true}, nil}
		return
	}
	powerVSCluster.updateCondition(true, infrav1beta2.VPCSubnetReadyCondition)

	// reconcile VPC security group
	vpcLog.Info("Reconciling VPC security group")
	if err := clusterScope.ReconcileVPCSecurityGroups(); err != nil {
		vpcLog.Error(err, "failed to reconcile VPC security groups")
		powerVSCluster.updateCondition(false, infrav1beta2.VPCSecurityGroupReadyCondition, infrav1beta2.VPCSecurityGroupReconciliationFailedReason, capiv1beta1.ConditionSeverityError, err.Error())
		ch <- reconcileResult{reconcile.Result{}, err}
		return
	}
	powerVSCluster.updateCondition(true, infrav1beta2.VPCSecurityGroupReadyCondition)

	// reconcile LoadBalancer
	vpcLog.Info("Reconciling VPC load balancers")
	if loadBalancerReady, err := clusterScope.ReconcileLoadBalancers(); err != nil {
		vpcLog.Error(err, "failed to reconcile VPC load balancers")
		powerVSCluster.updateCondition(false, infrav1beta2.LoadBalancerReadyCondition, infrav1beta2.LoadBalancerReconciliationFailedReason, capiv1beta1.ConditionSeverityError, err.Error())
		ch <- reconcileResult{reconcile.Result{}, err}
		return
	} else if loadBalancerReady {
		powerVSCluster.updateCondition(true, infrav1beta2.LoadBalancerReadyCondition)
		return
	}
	// Do not want to block the reconciliation of other resources like setting up TG and COS, so skipping the requeue and only logging the info.
	vpcLog.Info("VPC load balancer creation is pending")
}

func (r *IBMPowerVSClusterReconciler) reconcile(clusterScope *scope.PowerVSClusterScope) (ctrl.Result, error) { //nolint:gocyclo
	if controllerutil.AddFinalizer(clusterScope.IBMPowerVSCluster, infrav1beta2.IBMPowerVSClusterFinalizer) {
		return ctrl.Result{}, nil
	}

	// check for annotation set for cluster resource and decide on proceeding with infra creation.
	// do not proceed further if "powervs.cluster.x-k8s.io/create-infra=true" annotation is not set.
	if !scope.CheckCreateInfraAnnotation(*clusterScope.IBMPowerVSCluster) {
		clusterScope.IBMPowerVSCluster.Status.Ready = true
		return ctrl.Result{}, nil
	}

	// validate PER availability for the PowerVS zone, proceed further only if PowerVS zone support PER.
	// more information about PER can be found here: https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-per
	if err := clusterScope.IsPowerVSZoneSupportsPER(); err != nil {
		clusterScope.Error(err, "error checking PER capability for PowerVS zone")
		return reconcile.Result{}, err
	}

	// reconcile service resource group
	clusterScope.Info("Reconciling resource group")
	if err := clusterScope.ReconcileResourceGroup(); err != nil {
		clusterScope.Error(err, "failed to reconcile resource group")
		return reconcile.Result{}, err
	}

	powerVSCluster := &powerVSCluster{
		cluster: clusterScope.IBMPowerVSCluster,
	}

	var wg sync.WaitGroup
	ch := make(chan reconcileResult)

	// reconcile PowerVS resources
	wg.Add(1)
	go r.reconcilePowerVSResources(clusterScope, powerVSCluster, ch, &wg)

	// reconcile VPC
	wg.Add(1)
	go r.reconcileVPCResources(clusterScope, powerVSCluster, ch, &wg)

	// wait for above reconcile to complete and close the channel
	go func() {
		wg.Wait()
		close(ch)
	}()

	var requeue bool
	var errList []error
	// receive return values from the channel and decide the requeue
	for val := range ch {
		if val.Requeue {
			requeue = true
		}
		if val.error != nil {
			errList = append(errList, val.error)
		}
	}

	if requeue && len(errList) > 1 {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, kerrors.NewAggregate(errList)
	} else if requeue {
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	} else if len(errList) > 1 {
		return ctrl.Result{}, kerrors.NewAggregate(errList)
	}

	// reconcile Transit Gateway
	clusterScope.Info("Reconciling Transit Gateway")
	if requeue, err := clusterScope.ReconcileTransitGateway(); err != nil {
		clusterScope.Error(err, "failed to reconcile transit gateway")
		conditions.MarkFalse(powerVSCluster.cluster, infrav1beta2.TransitGatewayReadyCondition, infrav1beta2.TransitGatewayReconciliationFailedReason, capiv1beta1.ConditionSeverityError, err.Error())
		return reconcile.Result{}, err
	} else if requeue {
		clusterScope.Info("Setting up Transit gateway is pending, requeuing")
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	}
	conditions.MarkTrue(powerVSCluster.cluster, infrav1beta2.TransitGatewayReadyCondition)

	// reconcile COSInstance
	if clusterScope.IBMPowerVSCluster.Spec.Ignition != nil {
		clusterScope.Info("Reconciling COS service instance")
		if err := clusterScope.ReconcileCOSInstance(); err != nil {
			conditions.MarkFalse(powerVSCluster.cluster, infrav1beta2.COSInstanceReadyCondition, infrav1beta2.COSInstanceReconciliationFailedReason, capiv1beta1.ConditionSeverityError, err.Error())
			return reconcile.Result{}, err
		}
		conditions.MarkTrue(powerVSCluster.cluster, infrav1beta2.COSInstanceReadyCondition)
	}

	var networkReady, loadBalancerReady bool
	for _, cond := range clusterScope.IBMPowerVSCluster.Status.Conditions {
		if cond.Type == infrav1beta2.NetworkReadyCondition && cond.Status == corev1.ConditionTrue {
			networkReady = true
		}
		if cond.Type == infrav1beta2.LoadBalancerReadyCondition && cond.Status == corev1.ConditionTrue {
			loadBalancerReady = true
		}
	}

	if !networkReady || !loadBalancerReady {
		clusterScope.Info("Network or LoadBalancer still not ready, requeuing")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	// update cluster object with loadbalancer host
	loadBalancer := clusterScope.PublicLoadBalancer()
	if loadBalancer == nil {
		return reconcile.Result{}, fmt.Errorf("failed to fetch public loadbalancer")
	}

	clusterScope.Info("Getting load balancer host")
	hostName := clusterScope.GetLoadBalancerHostName(loadBalancer.Name)
	if hostName == nil || *hostName == "" {
		clusterScope.Info("LoadBalancer hostname is not yet available, requeuing")
		return reconcile.Result{RequeueAfter: time.Minute}, nil
	}

	clusterScope.IBMPowerVSCluster.Spec.ControlPlaneEndpoint.Host = *clusterScope.GetLoadBalancerHostName(loadBalancer.Name)
	clusterScope.IBMPowerVSCluster.Spec.ControlPlaneEndpoint.Port = clusterScope.APIServerPort()
	clusterScope.IBMPowerVSCluster.Status.Ready = true
	return ctrl.Result{}, nil
}

func (r *IBMPowerVSClusterReconciler) reconcileDelete(ctx context.Context, clusterScope *scope.PowerVSClusterScope) (ctrl.Result, error) {
	cluster := clusterScope.IBMPowerVSCluster

	if result, err := r.deleteIBMPowerVSImage(ctx, clusterScope); err != nil || !result.IsZero() {
		return result, err
	}

	// check for annotation set for cluster resource and decide on proceeding with infra deletion.
	if !scope.CheckCreateInfraAnnotation(*clusterScope.IBMPowerVSCluster) {
		controllerutil.RemoveFinalizer(cluster, infrav1beta2.IBMPowerVSClusterFinalizer)
		return ctrl.Result{}, nil
	}

	clusterScope.Info("Reconciling IBMPowerVSCluster delete")
	allErrs := []error{}
	clusterScope.IBMPowerVSClient.WithClients(powervs.ServiceOptions{CloudInstanceID: clusterScope.GetServiceInstanceID()})

	clusterScope.Info("Clean up Transit Gateway")
	if requeue, err := clusterScope.DeleteTransitGateway(); err != nil {
		allErrs = append(allErrs, errors.Wrapf(err, "failed to delete transit gateway"))
	} else if requeue {
		clusterScope.Info("Cleaning up transit gateway is pending, requeuing")
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	clusterScope.Info("Deleting VPC load balancer")
	if requeue, err := clusterScope.DeleteLoadBalancer(); err != nil {
		allErrs = append(allErrs, errors.Wrapf(err, "failed to delete VPC load balancer"))
	} else if requeue {
		clusterScope.Info("VPC load balancer deletion is pending, requeuing")
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	clusterScope.Info("Deleting VPC security group")
	if err := clusterScope.DeleteVPCSecurityGroups(); err != nil {
		allErrs = append(allErrs, errors.Wrapf(err, "failed to delete VPC subnet"))
	}

	clusterScope.Info("Deleting VPC subnet")
	if requeue, err := clusterScope.DeleteVPCSubnet(); err != nil {
		allErrs = append(allErrs, errors.Wrapf(err, "failed to delete VPC subnet"))
	} else if requeue {
		clusterScope.Info("VPC subnet deletion is pending, requeuing")
		return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
	}

	clusterScope.Info("Deleting VPC")
	if requeue, err := clusterScope.DeleteVPC(); err != nil {
		allErrs = append(allErrs, errors.Wrapf(err, "failed to delete VPC"))
	} else if requeue {
		clusterScope.Info("VPC deletion is pending, requeuing")
		return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
	}

	clusterScope.Info("Deleting DHCP server")
	if err := clusterScope.DeleteDHCPServer(); err != nil {
		allErrs = append(allErrs, errors.Wrapf(err, "failed to delete DHCP server"))
	}

	clusterScope.Info("Deleting Power VS service instance")
	if requeue, err := clusterScope.DeleteServiceInstance(); err != nil {
		allErrs = append(allErrs, errors.Wrapf(err, "failed to delete Power VS service instance"))
	} else if requeue {
		clusterScope.Info("PowerVS service instance deletion is pending, requeuing")
		return reconcile.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	if clusterScope.IBMPowerVSCluster.Spec.Ignition != nil {
		clusterScope.Info("Deleting COS service instance")
		if err := clusterScope.DeleteCOSInstance(); err != nil {
			allErrs = append(allErrs, errors.Wrapf(err, "failed to delete COS service instance"))
		}
	}

	if len(allErrs) > 0 {
		clusterScope.Error(kerrors.NewAggregate(allErrs), "failed to delete IBMPowerVSCluster")
		return ctrl.Result{}, kerrors.NewAggregate(allErrs)
	}

	clusterScope.Info("IBMPowerVSCluster deletion completed")
	controllerutil.RemoveFinalizer(cluster, infrav1beta2.IBMPowerVSClusterFinalizer)
	return ctrl.Result{}, nil
}

func (r *IBMPowerVSClusterReconciler) deleteIBMPowerVSImage(ctx context.Context, clusterScope *scope.PowerVSClusterScope) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	cluster := clusterScope.IBMPowerVSCluster
	descendants, err := r.listDescendants(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to list descendants")
		return reconcile.Result{}, err
	}

	// since we are avoiding using cache for IBMPowerVSCluster the Type meta of the retrieved object will be empty
	// explicitly setting here to filter children
	if gvk := cluster.GetObjectKind().GroupVersionKind(); gvk.Empty() {
		gvk, err := r.GroupVersionKindFor(cluster)
		if err != nil {
			log.Error(err, "Failed to get GVK of cluster")
			return reconcile.Result{}, err
		}
		cluster.SetGroupVersionKind(gvk)
	}

	children, err := descendants.filterOwnedDescendants(cluster)
	if err != nil {
		log.Error(err, "Failed to extract direct descendants")
		return reconcile.Result{}, err
	}

	if len(children) > 0 {
		log.Info("Cluster still has children - deleting them first", "count", len(children))

		var errs []error

		for _, child := range children {
			if !child.GetDeletionTimestamp().IsZero() {
				// Don't handle deleted child.
				continue
			}
			gvk := child.GetObjectKind().GroupVersionKind().String()

			log.Info("Deleting child object", "gvk", gvk, "name", child.GetName())
			if err := r.Client.Delete(ctx, child); err != nil {
				err = fmt.Errorf("error deleting cluster %s/%s: failed to delete %s %s: %w", cluster.Namespace, cluster.Name, gvk, child.GetName(), err)
				log.Error(err, "Error deleting resource", "gvk", gvk, "name", child.GetName())
				errs = append(errs, err)
			}
		}

		if len(errs) > 0 {
			return ctrl.Result{}, kerrors.NewAggregate(errs)
		}
	}

	if descendantCount := descendants.length(); descendantCount > 0 {
		indirect := descendantCount - len(children)
		log.Info("Cluster still has descendants - need to requeue", "descendants", descendants.descendantNames(), "indirect descendants count", indirect)
		// Requeue so we can check the next time to see if there are still any descendants left.
		return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
	}
	return ctrl.Result{}, nil
}

type clusterDescendants struct {
	ibmPowerVSImages infrav1beta2.IBMPowerVSImageList
}

// length returns the number of descendants.
func (c *clusterDescendants) length() int {
	return len(c.ibmPowerVSImages.Items)
}

func (c *clusterDescendants) descendantNames() string {
	descendants := make([]string, 0)
	ibmPowerVSImageNames := make([]string, len(c.ibmPowerVSImages.Items))
	for i, ibmPowerVSImage := range c.ibmPowerVSImages.Items {
		ibmPowerVSImageNames[i] = ibmPowerVSImage.Name
	}
	if len(ibmPowerVSImageNames) > 0 {
		descendants = append(descendants, "IBM Powervs Images: "+strings.Join(ibmPowerVSImageNames, ","))
	}

	return strings.Join(descendants, ";")
}

// listDescendants returns a list of all IBMPowerVSImages for the cluster.
func (r *IBMPowerVSClusterReconciler) listDescendants(ctx context.Context, cluster *infrav1beta2.IBMPowerVSCluster) (clusterDescendants, error) {
	var descendants clusterDescendants

	listOptions := []client.ListOption{
		client.InNamespace(cluster.Namespace),
		client.MatchingLabels(map[string]string{capiv1beta1.ClusterNameLabel: cluster.Name}),
	}

	if err := r.Client.List(ctx, &descendants.ibmPowerVSImages, listOptions...); err != nil {
		return descendants, fmt.Errorf("failed to list IBMPowerVSImages for cluster %s/%s: %w", cluster.Namespace, cluster.Name, err)
	}

	return descendants, nil
}

// filterOwnedDescendants returns an array of runtime.Objects containing only those descendants that have the cluster
// as an owner reference.
func (c clusterDescendants) filterOwnedDescendants(cluster *infrav1beta2.IBMPowerVSCluster) ([]client.Object, error) {
	var ownedDescendants []client.Object
	eachFunc := func(o runtime.Object) error {
		obj := o.(client.Object)
		acc, err := meta.Accessor(obj)
		if err != nil {
			return nil //nolint:nilerr // We don't want to exit the EachListItem loop, just continue
		}

		if util.IsOwnedByObject(acc, cluster) {
			ownedDescendants = append(ownedDescendants, obj)
		}

		return nil
	}

	lists := []client.ObjectList{
		&c.ibmPowerVSImages,
	}

	for _, list := range lists {
		if err := meta.EachListItem(list, eachFunc); err != nil {
			return nil, fmt.Errorf("error finding owned descendants of cluster %s/%s: %w", cluster.Namespace, cluster.Name, err)
		}
	}

	return ownedDescendants, nil
}

// SetupWithManager creates a new IBMPowerVSCluster controller for a manager.
func (r *IBMPowerVSClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1beta2.IBMPowerVSCluster{}).
		WithEventFilter(predicates.ResourceIsNotExternallyManaged(ctrl.LoggerFrom(ctx))).
		Complete(r)
}
