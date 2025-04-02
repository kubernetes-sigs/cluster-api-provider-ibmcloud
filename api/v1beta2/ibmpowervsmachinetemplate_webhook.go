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

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-ibmpowervsmachinetemplate,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsmachinetemplates,verbs=create;update,versions=v1beta2,name=mibmpowervsmachinetemplate.kb.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
//+kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-ibmpowervsmachinetemplate,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsmachinetemplates,versions=v1beta2,name=vibmpowervsmachinetemplate.kb.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

func (r *IBMPowerVSMachineTemplate) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&IBMPowerVSMachineTemplate{}).
		WithValidator(r).
		WithDefaulter(r).
		Complete()
}

var _ webhook.CustomDefaulter = &IBMPowerVSMachineTemplate{}
var _ webhook.CustomValidator = &IBMPowerVSMachineTemplate{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (r *IBMPowerVSMachineTemplate) Default(_ context.Context, obj runtime.Object) error {
	objValue, ok := obj.(*IBMPowerVSMachineTemplate)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a IBMPowerVSMachineTemplate but got a %T", obj))
	}
	defaultIBMPowerVSMachineSpec(&objValue.Spec.Template.Spec)
	return nil
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMPowerVSMachineTemplate) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	objValue, ok := obj.(*IBMPowerVSMachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IBMPowerVSMachineTemplate but got a %T", obj))
	}
	return objValue.validateIBMPowerVSMachineTemplate()
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMPowerVSMachineTemplate) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (warnings admission.Warnings, err error) {
	objValue, ok := newObj.(*IBMPowerVSMachineTemplate)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IBMPowerVSMachineTemplate but got a %T", newObj))
	}
	return objValue.validateIBMPowerVSMachineTemplate()
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMPowerVSMachineTemplate) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func (r *IBMPowerVSMachineTemplate) validateIBMPowerVSMachineTemplate() (admission.Warnings, error) {
	var allErrs field.ErrorList
	if err := r.validateIBMPowerVSMachineTemplateNetwork(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := r.validateIBMPowerVSMachineTemplateImage(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := r.validateIBMPowerVSMachineTemplateMemory(); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := r.validateIBMPowerVSMachineTemplateProcessors(); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "IBMPowerVSMachineTemplate"},
		r.Name, allErrs)
}

func (r *IBMPowerVSMachineTemplate) validateIBMPowerVSMachineTemplateNetwork() *field.Error {
	if res, err := validateIBMPowerVSNetworkReference(r.Spec.Template.Spec.Network); !res {
		return err
	}
	return nil
}

func (r *IBMPowerVSMachineTemplate) validateIBMPowerVSMachineTemplateImage() *field.Error {
	mt := r.Spec.Template

	if mt.Spec.Image == nil && mt.Spec.ImageRef == nil {
		return field.Invalid(field.NewPath(""), "", "One of - Image or ImageRef must be specified")
	}

	if mt.Spec.Image != nil && mt.Spec.ImageRef != nil {
		return field.Invalid(field.NewPath(""), "", "Only one of - Image or ImageRef maybe be specified")
	}

	if mt.Spec.Image != nil {
		if res, err := validateIBMPowerVSResourceReference(*mt.Spec.Image, "Image"); !res {
			return err
		}
	}

	return nil
}

func (r *IBMPowerVSMachineTemplate) validateIBMPowerVSMachineTemplateMemory() *field.Error {
	if res := validateIBMPowerVSMemoryValues(r.Spec.Template.Spec.MemoryGiB); !res {
		return field.Invalid(field.NewPath("spec", "template", "spec", "memoryGiB"), r.Spec.Template.Spec.MemoryGiB, "Invalid Memory value - must be a positive integer no lesser than 2")
	}
	return nil
}

func (r *IBMPowerVSMachineTemplate) validateIBMPowerVSMachineTemplateProcessors() *field.Error {
	if res := validateIBMPowerVSProcessorValues(r.Spec.Template.Spec.Processors); !res {
		return field.Invalid(field.NewPath("spec", "template", "spec", "processors"), r.Spec.Template.Spec.Processors, "Invalid Processors value - must be non-empty and positive floating-point number no lesser than 0.25")
	}
	return nil
}
