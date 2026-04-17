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

package webhooks

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

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
)

//+kubebuilder:webhook:path=/mutate-infrastructure-cluster-x-k8s-io-v1beta2-ibmpowervsmachine,mutating=true,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsmachines,verbs=create;update,versions=v1beta2,name=mibmpowervsmachine.kb.io,sideEffects=None,admissionReviewVersions=v1;v1beta1
//+kubebuilder:webhook:verbs=create;update,path=/validate-infrastructure-cluster-x-k8s-io-v1beta2-ibmpowervsmachine,mutating=false,failurePolicy=fail,groups=infrastructure.cluster.x-k8s.io,resources=ibmpowervsmachines,versions=v1beta2,name=vibmpowervsmachine.kb.io,sideEffects=None,admissionReviewVersions=v1;v1beta1

func (r *IBMPowerVSMachine) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(&infrav1.IBMPowerVSMachine{}).
		WithValidator(r).
		WithDefaulter(r).
		Complete()
}

// IBMPowerVSMachine implements a validation and defaulting webhook for IBMPowerVSMachine.
type IBMPowerVSMachine struct{}

var _ webhook.CustomDefaulter = &IBMPowerVSMachine{}
var _ webhook.CustomValidator = &IBMPowerVSMachine{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the type.
func (r *IBMPowerVSMachine) Default(_ context.Context, obj runtime.Object) error {
	objValue, ok := obj.(*infrav1.IBMPowerVSMachine)
	if !ok {
		return apierrors.NewBadRequest(fmt.Sprintf("expected a IBMPowerVSMachine but got a %T", obj))
	}
	defaultIBMPowerVSMachineSpec(&objValue.Spec)
	return nil
}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMPowerVSMachine) ValidateCreate(_ context.Context, obj runtime.Object) (admission.Warnings, error) {
	objValue, ok := obj.(*infrav1.IBMPowerVSMachine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IBMPowerVSMachine but got a %T", obj))
	}
	return validateIBMPowerVSMachine(objValue)
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMPowerVSMachine) ValidateUpdate(_ context.Context, _, newObj runtime.Object) (warnings admission.Warnings, err error) {
	newObjValue, ok := newObj.(*infrav1.IBMPowerVSMachine)
	if !ok {
		return nil, apierrors.NewBadRequest(fmt.Sprintf("expected a IBMPowerVSMachine but got a %T", newObj))
	}
	return validateIBMPowerVSMachine(newObjValue)
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type.
func (r *IBMPowerVSMachine) ValidateDelete(_ context.Context, _ runtime.Object) (admission.Warnings, error) {
	return nil, nil
}

func validateIBMPowerVSMachine(machine *infrav1.IBMPowerVSMachine) (admission.Warnings, error) {
	var allErrs field.ErrorList
	if err := validateIBMPowerVSMachineNetwork(machine); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateIBMPowerVSMachineImage(machine); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateIBMPowerVSMachineMemory(machine); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateIBMPowerVSMachineProcessors(machine); err != nil {
		allErrs = append(allErrs, err)
	}
	if err := validateIBMPowerVSMachineSystemType(machine); err != nil {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) == 0 {
		return nil, nil
	}

	return nil, apierrors.NewInvalid(
		schema.GroupKind{Group: "infrastructure.cluster.x-k8s.io", Kind: "IBMPowerVSMachine"},
		machine.Name, allErrs)
}

func validateIBMPowerVSMachineNetwork(machine *infrav1.IBMPowerVSMachine) *field.Error {
	if res, err := validateIBMPowerVSNetworkReference(machine.Spec.Network); !res {
		return err
	}
	return nil
}

func validateIBMPowerVSMachineImage(machine *infrav1.IBMPowerVSMachine) *field.Error {
	if machine.Spec.Image == nil && machine.Spec.ImageRef == nil {
		return field.Invalid(field.NewPath(""), "", "One of - Image or ImageRef must be specified")
	}

	if machine.Spec.Image != nil && machine.Spec.ImageRef != nil {
		return field.Invalid(field.NewPath(""), "", "Only one of - Image or ImageRef maybe be specified")
	}

	if machine.Spec.Image != nil {
		if res, err := validateIBMPowerVSResourceReference(*machine.Spec.Image, "Image"); !res {
			return err
		}
	}
	return nil
}

func validateIBMPowerVSMachineMemory(machine *infrav1.IBMPowerVSMachine) *field.Error {
	if res := validateIBMPowerVSMemoryValues(machine.Spec.MemoryGiB); !res {
		return field.Invalid(field.NewPath("spec", "memoryGiB"), machine.Spec.MemoryGiB, "Invalid Memory value - must a positive integer no lesser than 2")
	}
	return nil
}

func validateIBMPowerVSMachineProcessors(machine *infrav1.IBMPowerVSMachine) *field.Error {
	if res := validateIBMPowerVSProcessorValues(machine.Spec.Processors); !res {
		return field.Invalid(field.NewPath("spec", "processors"), machine.Spec.Processors, "Invalid Processors value - must be non-empty and positive floating-point number no lesser than 0.25")
	}
	return nil
}

// validateIBMPowerVSMachineSystemType validates the system type against dynamically fetched supported types.
// NOTE: This validation requires a PISession which is not available in the webhook context.
// For now, we use a fallback list of known system types.
// In production I suggest:
// 1. Implementing a caching mechanism that periodically fetches supported types
// 2. Or use a ConfigMap to store the supported types list (still hardcoded)
// 3. Or accept any system type and let the controller handle validation
func validateIBMPowerVSMachineSystemType(machine *infrav1.IBMPowerVSMachine) *field.Error {
	// If SystemType is empty, it's optional and valid
	if machine.Spec.SystemType == "" {
		return nil
	}

	// Fallback to known system types since we don't have PISession in webhook context
	supportedTypes := []string{"s922", "e980", "s1022", "s1122", "e1050", "e1080"}

	// Check if the provided system type is in the supported list
	for _, validType := range supportedTypes {
		if machine.Spec.SystemType == validType {
			return nil
		}
	}

	return field.Invalid(
		field.NewPath("spec", "systemType"),
		machine.Spec.SystemType,
		fmt.Sprintf("must be one of: %v", supportedTypes),
	)
}
