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

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-ibmpowervsmachine,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsmachines,verbs=create;update,versions=v1beta2,name=mibmpowervsmachine.kb.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
//+kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-ibmpowervsmachine,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsmachines,versions=v1beta2,name=vibmpowervsmachine.kb.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

func (r *IBMPowerVSMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&IBMPowerVSMachine{}).
		WithValidator(r).
		WithDefaulter(r).
		Complete()
}

var _ webhook.CustomDefaulter = &IBMPowerVSMachine{}
var _ webhook.CustomValidator = &IBMPowerVSMachine{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (r *IBMPowerVSMachine) Default(_ context.Context, obj runtime.Object) error {
	objValue, ok := obj.(*IBMPowerVSMachine)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a IBMPowerVSMachine but got a %T", obj))
	}
	defaultIBMPowerVSMachineSpec(&objValue.Spec)
	return nil
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMPowerVSMachine) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	objValue, ok := obj.(*IBMPowerVSMachine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IBMPowerVSMachine but got a %T", obj))
	}
	return objValue.validateIBMPowerVSMachine()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMPowerVSMachine) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (warnings admission.Warnings, err error) {
	newObjValue, ok := newObj.(*IBMPowerVSMachine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IBMPowerVSMachine but got a %T", newObj))
	}
	return newObjValue.validateIBMPowerVSMachine()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMPowerVSMachine) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (r *IBMPowerVSMachine) validateIBMPowerVSMachine() (admission.Warnings, error) {
	var allErrs field.ErrorList
	if err := r.validateIBMPowerVSMachineNetwork(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := r.validateIBMPowerVSMachineImage(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := r.validateIBMPowerVSMachineMemory(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := r.validateIBMPowerVSMachineProcessors(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "IBMPowerVSMachine"},
		r.Name, allErrs)
}

func (r *IBMPowerVSMachine) validateIBMPowerVSMachineNetwork() *field.Error {
	if res, err := validateIBMPowerVSNetworkReference(r.Spec.Network); !res {
		return err
	}
	return nil
}

func (r *IBMPowerVSMachine) validateIBMPowerVSMachineImage() *field.Error {
	if r.Spec.Image == nil && r.Spec.ImageRef == nil {
		return field.Invalid(field.NewPath(""), "", "One of - Image or ImageRef must be specified")
	}

	if r.Spec.Image != nil && r.Spec.ImageRef != nil {
		return field.Invalid(field.NewPath(""), "", "Only one of - Image or ImageRef maybe be specified")
	}

	if r.Spec.Image != nil {
		if res, err := validateIBMPowerVSResourceReference(*r.Spec.Image, "Image"); !res {
			return err
		}
	}
	return nil
}

func (r *IBMPowerVSMachine) validateIBMPowerVSMachineMemory() *field.Error {
	if res := validateIBMPowerVSMemoryValues(r.Spec.MemoryGiB); !res {
		return field.Invalid(field.NewPath("spec", "memoryGiB"), r.Spec.MemoryGiB, "Invalid Memory value - must a positive integer no lesser than 2")
	}
	return nil
}

func (r *IBMPowerVSMachine) validateIBMPowerVSMachineProcessors() *field.Error {
	if res := validateIBMPowerVSProcessorValues(r.Spec.Processors); !res {
		return field.Invalid(field.NewPath("spec", "processors"), r.Spec.Processors, "Invalid Processors value - must be non-empty and positive floating-point number no lesser than 0.25")
	}
	return nil
}
