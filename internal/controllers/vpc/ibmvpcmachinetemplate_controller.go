/*
Copyright 2023 The Kubernetes Authors.

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

package vpc

import (
	"context"
	"fmt"
	"reflect"
	"strconv"

	"github.com/IBM/vpc-go-sdk/vpcv1"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta1patch "sigs.k8s.io/cluster-api/util/deprecated/v1beta1/patch" //nolint:staticcheck

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/vpc/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/endpoints"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc"
)

// IBMVPCMachineTemplateReconciler reconciles a IBMVPCMachineTemplate object.
type IBMVPCMachineTemplateReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	ServiceEndpoint []endpoints.ServiceEndpoint
}

func (r *IBMVPCMachineTemplateReconciler) SetupWithManager(_ context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1.IBMVPCMachineTemplate{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcmachinetemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcmachinetemplates/status,verbs=get;update;patch

func (r *IBMVPCMachineTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.Info("Reconciling IBMVPCMachineTemplate")

	var machineTemplate infrav1.IBMVPCMachineTemplate
	if err := r.Get(ctx, req.NamespacedName, &machineTemplate); err != nil {
		log.Error(err, "Unable to fetch ibmvpcmachinetemplate")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	region := endpoints.ConstructRegionFromZone(machineTemplate.Spec.Template.Spec.Zone)

	// Fetch the service endpoint.
	svcEndpoint := endpoints.FetchVPCEndpoint(region, r.ServiceEndpoint)

	vpcClient, err := vpc.NewService(svcEndpoint)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create IBM VPC client: %w", err)
	}

	return r.reconcileNormal(ctx, vpcClient, machineTemplate)
}

func (r *IBMVPCMachineTemplateReconciler) reconcileNormal(ctx context.Context, vpcClient vpc.Vpc, machineTemplate infrav1.IBMVPCMachineTemplate) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	helper, err := v1beta1patch.NewHelper(&machineTemplate, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}

	options := &vpcv1.GetInstanceProfileOptions{}
	options.SetName(machineTemplate.Spec.Template.Spec.Profile)
	profileDetails, _, err := vpcClient.GetInstanceProfile(options)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to fetch profile Details: %w", err)
	}

	if profileDetails == nil {
		return ctrl.Result{}, fmt.Errorf("failed to find profileDetails")
	}

	log.V(3).Info("Profile Details:", "profileDetails", profileDetails)

	capacity := make(corev1.ResourceList)
	if memGiB, ok := profileMemoryGiB(profileDetails.Memory); ok {
		capacity[corev1.ResourceMemory] = resource.MustParse(strconv.FormatInt(memGiB, 10) + "G")
	}
	if cpus, ok := profileVcpuCount(profileDetails.VcpuCount); ok {
		capacity[corev1.ResourceCPU] = resource.MustParse(strconv.FormatInt(cpus, 10))
	}
	if len(capacity) == 0 {
		log.V(3).Info("Unable to determine capacity from instance profile", "profile", machineTemplate.Spec.Template.Spec.Profile)
	}

	log.V(3).Info("Calculated capacity for machine template", "capacity", capacity)
	if !reflect.DeepEqual(machineTemplate.Status.Capacity, capacity) {
		machineTemplate.Status.Capacity = capacity
		if err := helper.Patch(ctx, &machineTemplate); err != nil {
			if !apierrors.IsNotFound(err) {
				log.Error(err, "Failed to patch machineTemplate")
				return ctrl.Result{}, err
			}
		}
	}
	log.V(3).Info("Machine template status", "status", machineTemplate.Status)
	return ctrl.Result{}, nil
}

// profileMemoryGiB returns the memory in GiB of an instance profile, falling back to the
// default for enum and range profiles. It returns false if the profile has neither, as is
// the case for dependent profiles.
func profileMemoryGiB(memory vpcv1.InstanceProfileMemoryIntf) (int64, bool) {
	switch m := memory.(type) {
	case *vpcv1.InstanceProfileMemory:
		if m != nil && m.Value != nil {
			return *m.Value, true
		}
		if m != nil && m.Default != nil {
			return *m.Default, true
		}
	case *vpcv1.InstanceProfileMemoryFixed:
		if m != nil && m.Value != nil {
			return *m.Value, true
		}
	case *vpcv1.InstanceProfileMemoryEnum:
		if m != nil && m.Default != nil {
			return *m.Default, true
		}
	case *vpcv1.InstanceProfileMemoryRange:
		if m != nil && m.Default != nil {
			return *m.Default, true
		}
	}
	return 0, false
}

// profileVcpuCount returns the vCPU count of an instance profile, in the same way as
// profileMemoryGiB.
func profileVcpuCount(vcpuCount vpcv1.InstanceProfileVcpuIntf) (int64, bool) {
	switch v := vcpuCount.(type) {
	case *vpcv1.InstanceProfileVcpu:
		if v != nil && v.Value != nil {
			return *v.Value, true
		}
		if v != nil && v.Default != nil {
			return *v.Default, true
		}
	case *vpcv1.InstanceProfileVcpuFixed:
		if v != nil && v.Value != nil {
			return *v.Value, true
		}
	case *vpcv1.InstanceProfileVcpuEnum:
		if v != nil && v.Default != nil {
			return *v.Default, true
		}
	case *vpcv1.InstanceProfileVcpuRange:
		if v != nil && v.Default != nil {
			return *v.Default, true
		}
	}
	return 0, false
}
