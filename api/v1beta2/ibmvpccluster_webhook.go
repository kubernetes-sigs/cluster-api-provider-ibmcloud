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

package v1beta2

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-ibmvpccluster,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcclusters,verbs=create;update,versions=v1beta2,name=mibmvpccluster.kb.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
//+kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-ibmvpccluster,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=ibmvpcclusters,versions=v1beta2,name=vibmvpccluster.kb.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

func (r *IBMVPCCluster) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&IBMVPCCluster{}).
		WithValidator(r).
		WithDefaulter(r).
		Complete()
}

var _ webhook.CustomDefaulter = &IBMVPCCluster{}
var _ webhook.CustomValidator = &IBMVPCCluster{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (r *IBMVPCCluster) Default(_ context.Context, _ runtime.Object) error {
	return nil
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMVPCCluster) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	objValue, ok := obj.(*IBMVPCCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IBMVPCCluster but got a %T", obj))
	}
	return objValue.validateIBMVPCCluster()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMVPCCluster) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (warnings admission.Warnings, err error) {
	objValue, ok := newObj.(*IBMVPCCluster)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IBMVPCCluster but got a %T", objValue))
	}
	return objValue.validateIBMVPCCluster()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMVPCCluster) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (r *IBMVPCCluster) validateIBMVPCCluster() (admission.Warnings, error) {
	var allErrs field.ErrorList
	if err := r.validateIBMVPCClusterControlPlane(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "IBMVPCCluster"},
		r.Name, allErrs)
}

func (r *IBMVPCCluster) validateIBMVPCClusterControlPlane() *field.Error {
	if r.Spec.ControlPlaneEndpoint.Host == "" && r.Spec.ControlPlaneLoadBalancer == nil {
		return field.Invalid(field.NewPath(""), "", "One of - ControlPlaneEndpoint or ControlPlaneLoadBalancer must be specified")
	}
	return nil
}
