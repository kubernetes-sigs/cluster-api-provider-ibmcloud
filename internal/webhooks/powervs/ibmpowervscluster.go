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

package powervs

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	regionUtil "github.com/ppc64le-cloud/powervs-utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/internal/genutil"
)

// Ensure IBMPowerVSCluster implements the typed webhook interfaces.
var (
	_ admission.Validator[*infrav1.IBMPowerVSCluster] = &IBMPowerVSCluster{}
	_ admission.Defaulter[*infrav1.IBMPowerVSCluster] = &IBMPowerVSCluster{}
)

const (
	infrastructureGroup = "infrastructure.cluster.x-k8s.io"
)

//+kubebuilder:webhook:verbs=create;update,path=/mutate-infrastructure-cluster-x-k8s-io-v1beta3-ibmpowervscluster,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsclusters,versions=v1beta3,name=mibmpowervscluster.kb.io,sideEffects=None,admissionReviewVersions=v1
//+kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta3-ibmpowervscluster,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsclusters,versions=v1beta3,name=vibmpowervscluster.kb.io,sideEffects=None,admissionReviewVersions=v1

func (r *IBMPowerVSCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr, &infrav1.IBMPowerVSCluster{}).
		WithValidator(r).
		WithDefaulter(r).
		Complete()
}

// IBMPowerVSCluster implements a validation and defaulting webhook for IBMPowerVSCluster.
type IBMPowerVSCluster struct{}

// Default implements webhook.Defaulter so a webhook will be registered for the type.
func (r *IBMPowerVSCluster) Default(_ context.Context, _ *infrav1.IBMPowerVSCluster) error {
	return nil
}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type.
func (r *IBMPowerVSCluster) ValidateCreate(_ context.Context, obj *infrav1.IBMPowerVSCluster) (admission.Warnings, error) {
	return validateIBMPowerVSCluster(nil, obj)
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type.
func (r *IBMPowerVSCluster) ValidateUpdate(_ context.Context, oldObj, newObj *infrav1.IBMPowerVSCluster) (warnings admission.Warnings, err error) {
	return validateIBMPowerVSCluster(oldObj, newObj)
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type.
func (r *IBMPowerVSCluster) ValidateDelete(_ context.Context, _ *infrav1.IBMPowerVSCluster) (admission.Warnings, error) {
	return nil, nil
}

func validateIBMPowerVSCluster(oldCluster, newCluster *infrav1.IBMPowerVSCluster) (admission.Warnings, error) {
	var allErrs field.ErrorList
	if err := validateIBMPowerVSClusterNetwork(newCluster); err != nil {
		allErrs = append(allErrs, err)
	}

	if err := validateIBMPowerVSClusterCreateInfraPrereq(newCluster); err != nil {
		allErrs = append(allErrs, err...)
	}
	// Need not validate for create operation
	if oldCluster != nil {
		if err := validateAdditionalListenerSelector(newCluster, oldCluster); err != nil {
			allErrs = append(allErrs, err...)
		}
	}

	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: infrastructureGroup, Kind: "IBMPowerVSCluster"},
		newCluster.Name, allErrs)
}

func validateIBMPowerVSClusterNetwork(cluster *infrav1.IBMPowerVSCluster) *field.Error {
	// Validate NetworkSource based on Type
	switch cluster.Spec.Network.Type {
	case infrav1.SourceTypeReference:
		// Validate that Reference has either ID or Name
		if cluster.Spec.Network.Reference.ID == "" && cluster.Spec.Network.Reference.Name == "" {
			return field.Invalid(field.NewPath("spec.network.reference"), cluster.Spec.Network.Reference, "either ID or Name must be provided when type is Reference")
		}
		// Ensure Provision is not set when Type is Reference
		if cluster.Spec.Network.Provision.DHCPServer.Name != "" || cluster.Spec.Network.Provision.DHCPServer.CIDR != "" {
			return field.Invalid(field.NewPath("spec.network.provision"), cluster.Spec.Network.Provision, "provision must not be set when type is Reference")
		}
	case infrav1.SourceTypeProvision:
		// Ensure Reference is not set when Type is Provision
		if cluster.Spec.Network.Reference.ID != "" || cluster.Spec.Network.Reference.Name != "" {
			return field.Invalid(field.NewPath("spec.network.reference"), cluster.Spec.Network.Reference, "reference must not be set when type is Provision")
		}
	default:
		// Catch empty strings or invalid enum values and explicitly list what is allowed
		validTypes := []string{string(infrav1.SourceTypeReference), string(infrav1.SourceTypeProvision)}
		return field.NotSupported(field.NewPath("spec.network.type"), cluster.Spec.Network.Type, validTypes)
	}
	return nil
}

func validateIBMPowerVSClusterLoadBalancers(cluster *infrav1.IBMPowerVSCluster) (allErrs field.ErrorList) {
	if err := validateIBMPowerVSClusterLoadBalancerNames(cluster); err != nil {
		allErrs = append(allErrs, err...)
	}

	if len(cluster.Spec.LoadBalancers) == 0 {
		return allErrs
	}

	for _, loadBalancer := range cluster.Spec.LoadBalancers {
		if *loadBalancer.Public {
			return allErrs
		}
	}

	return append(allErrs, field.Invalid(field.NewPath("spec.LoadBalancers"), cluster.Spec.LoadBalancers, "Expect atleast one of the load balancer to be public"))
}

func validateIBMPowerVSClusterLoadBalancerNames(cluster *infrav1.IBMPowerVSCluster) (allErrs field.ErrorList) {
	found := make(map[string]bool)
	for i, loadbalancer := range cluster.Spec.LoadBalancers {
		if loadbalancer.Name == "" {
			continue
		}

		if found[loadbalancer.Name] {
			allErrs = append(allErrs, field.Duplicate(field.NewPath("spec", fmt.Sprintf("loadbalancers[%d]", i)), map[string]interface{}{"Name": loadbalancer.Name}))
			continue
		}
		found[loadbalancer.Name] = true
	}

	return allErrs
}

func validateIBMPowerVSClusterVPCSubnetNames(cluster *infrav1.IBMPowerVSCluster) (allErrs field.ErrorList) {
	found := make(map[string]bool)
	for i, subnet := range cluster.Spec.VPCSubnets {
		if subnet.Name == nil {
			continue
		}
		if found[*subnet.Name] {
			allErrs = append(allErrs, field.Duplicate(field.NewPath("spec", fmt.Sprintf("vpcSubnets[%d]", i)), map[string]interface{}{"Name": *subnet.Name}))
			continue
		}
		found[*subnet.Name] = true
	}

	return allErrs
}

func validateIBMPowerVSClusterTransitGateway(cluster *infrav1.IBMPowerVSCluster) *field.Error {
	if cluster.Spec.Zone == "" || cluster.Spec.VPC == nil || cluster.Spec.VPC.Region == nil {
		return nil
	}
	// TransitGateway is now a value type, check if Type is set to determine if it's configured
	if cluster.Spec.TransitGateway.Type == "" {
		return nil
	}
	// GlobalRouting is now in Provision field and is a string enum, not a bool pointer
	if cluster.Spec.TransitGateway.Type == infrav1.SourceTypeProvision {
		if _, globalRouting, _ := genutil.GetTransitGatewayLocationAndRouting(&cluster.Spec.Zone, cluster.Spec.VPC.Region); cluster.Spec.TransitGateway.Provision.GlobalRouting == infrav1.TransitGatewayRoutingLocal && globalRouting != nil && *globalRouting {
			return field.Invalid(field.NewPath("spec.transitGateway.provision.globalRouting"), cluster.Spec.TransitGateway.Provision.GlobalRouting, "global routing is required since PowerVS and VPC region are from different region")
		}
	}
	return nil
}

func validateIBMPowerVSClusterCreateInfraPrereq(cluster *infrav1.IBMPowerVSCluster) (allErrs field.ErrorList) {
	annotations := cluster.GetAnnotations()
	if len(annotations) == 0 {
		return nil
	}

	value, found := annotations[infrav1.CreateInfrastructureAnnotation]
	if !found {
		return nil
	}

	createInfra, err := strconv.ParseBool(value)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("annotations"), cluster.Annotations, "value of powervs.cluster.x-k8s.io/create-infra should be boolean"))
	}

	if !createInfra {
		return nil
	}

	if cluster.Spec.Zone == "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.zone"), cluster.Spec.Zone, "value of zone is empty"))
	}

	if cluster.Spec.Zone != "" && !regionUtil.ValidateZone(cluster.Spec.Zone) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.zone"), cluster.Spec.Zone, fmt.Sprintf("zone '%s' is not supported", cluster.Spec.Zone)))
	}

	if cluster.Spec.VPC == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.vpc"), cluster.Spec.VPC, "value of VPC is empty"))
	}

	if cluster.Spec.VPC != nil && cluster.Spec.VPC.Region == nil {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.vpc.region"), cluster.Spec.VPC.Region, "value of VPC region is empty"))
	}

	if cluster.Spec.VPC != nil && cluster.Spec.VPC.Region != nil && !regionUtil.ValidateVPCRegion(*cluster.Spec.VPC.Region) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.vpc.region"), cluster.Spec.VPC.Region, fmt.Sprintf("vpc region '%s' is not supported", *cluster.Spec.VPC.Region)))
	}

	if cluster.Spec.ResourceGroup.Type == "" {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec.resourceGroup"), cluster.Spec.ResourceGroup, "value of resource group is empty"))
	}
	if err := validateIBMPowerVSClusterVPCSubnetNames(cluster); err != nil {
		allErrs = append(allErrs, err...)
	}

	if err := validateIBMPowerVSClusterLoadBalancers(cluster); err != nil {
		allErrs = append(allErrs, err...)
	}

	if err := validateIBMPowerVSClusterTransitGateway(cluster); err != nil {
		allErrs = append(allErrs, err)
	}

	return allErrs
}

func validateAdditionalListenerSelector(newCluster, oldCluster *infrav1.IBMPowerVSCluster) (allErrs field.ErrorList) {
	newLoadBalancerListeners := map[string]metav1.LabelSelector{}
	for _, loadbalancer := range newCluster.Spec.LoadBalancers {
		for _, additionalListener := range loadbalancer.AdditionalListeners {
			var key string
			if additionalListener.Protocol != nil {
				key = fmt.Sprintf("%d-%s", additionalListener.Port, *additionalListener.Protocol)
			} else {
				// Use default protocol marker when protocol is not specified
				key = fmt.Sprintf("%d-<default>", additionalListener.Port)
			}
			newLoadBalancerListeners[key] = additionalListener.Selector
		}
	}
	for _, loadbalancer := range oldCluster.Spec.LoadBalancers {
		for _, additionalListener := range loadbalancer.AdditionalListeners {
			var key string
			if additionalListener.Protocol != nil {
				key = fmt.Sprintf("%d-%s", additionalListener.Port, *additionalListener.Protocol)
			} else {
				// Use default protocol marker when protocol is not specified
				key = fmt.Sprintf("%d-<default>", additionalListener.Port)
			}
			if selector, ok := newLoadBalancerListeners[key]; ok && !reflect.DeepEqual(selector, additionalListener.Selector) {
				allErrs = append(allErrs, field.Forbidden(
					field.NewPath("spec", "loadBalancers", "additionalListeners", "selector"),
					fmt.Sprintf("Selector is immutable for port %d", additionalListener.Port)))
			}
		}
	}
	return allErrs
}
