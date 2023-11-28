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

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/runtime"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api/util/patch"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
)

// IBMVPCMachineTemplateReconciler reconciles a IBMVPCMachineTemplate object.
type IBMVPCMachineTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *IBMVPCMachineTemplateReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrav1beta2.IBMVPCMachineTemplate{}).
		Complete(r)
}

// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcmachinetemplates,verbs=get;list;watch
// +kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcmachinetemplates/status,verbs=get;update;patch

func (r *IBMVPCMachineTemplateReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(1).Info("Reconciling IBMVPCMachineTemplate")

	var machineTemplate infrav1beta2.IBMVPCMachineTemplate
	if err := r.Get(ctx, req.NamespacedName, &machineTemplate); err != nil {
		log.Error(err, "Unable to fetch ibmvpcmachinetemplate")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	helper, err := patch.NewHelper(&machineTemplate, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to init patch helper: %w", err)
	}

	capacity, err := getIBMVPCMachineCapacity(machineTemplate)
	if err != nil {
		log.Error(err, "Failed to get capacity from the ibmvpcmachine template")
		return ctrl.Result{}, fmt.Errorf("failed to get capcity for machine template: %w", err)
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

func getIBMVPCMachineCapacity(machineTemplate infrav1beta2.IBMVPCMachineTemplate) (corev1.ResourceList, error) {
	capacity := make(corev1.ResourceList)
	regex := "\\S+[-]\\d+[x]\\d+\\S*$"
	re, err := regexp.Compile(regex)
	if err != nil {
		return nil, fmt.Errorf("failed to compile regular expression %s", regex)
	}
	if match := re.Match([]byte(machineTemplate.Spec.Template.Spec.Profile)); match {
		Profile := strings.Split(strings.Split(machineTemplate.Spec.Template.Spec.Profile, "-")[1], "x")
		capacity[corev1.ResourceCPU] = resource.MustParse(Profile[0])
		capacity[corev1.ResourceMemory] = resource.MustParse(fmt.Sprintf("%sG", Profile[1]))
		fmt.Printf("capacity : %+v", capacity)
		return capacity, nil
	}
	return nil, fmt.Errorf("invalid Profile")
}
