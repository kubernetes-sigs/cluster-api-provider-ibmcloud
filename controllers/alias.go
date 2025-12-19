/*
Copyright 2025 The Kubernetes Authors.

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

	"github.com/go-logr/logr"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	powervscontroller "sigs.k8s.io/cluster-api-provider-ibmcloud/internal/controllers/powervs"
	vpccontroller "sigs.k8s.io/cluster-api-provider-ibmcloud/internal/controllers/vpc"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/endpoints"
)

// IBMVPCClusterReconciler reonciles a IBMVPCCluster object.
type IBMVPCClusterReconciler struct {
	client.Client
	Log             logr.Logger
	Recorder        record.EventRecorder
	ServiceEndpoint []endpoints.ServiceEndpoint
	Scheme          *runtime.Scheme
}

func (r *IBMVPCClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return (&vpccontroller.IBMVPCClusterReconciler{
		Client:          r.Client,
		Log:             r.Log,
		Recorder:        r.Recorder,
		ServiceEndpoint: r.ServiceEndpoint,
		Scheme:          r.Scheme,
	}).SetupWithManager(ctx, mgr)
}

// IBMVPCMachineReconciler reconciles a IBMVPCMachine object.
type IBMVPCMachineReconciler struct {
	client.Client
	Log             logr.Logger
	Recorder        record.EventRecorder
	ServiceEndpoint []endpoints.ServiceEndpoint
	Scheme          *runtime.Scheme
}

func (r *IBMVPCMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return (&vpccontroller.IBMVPCMachineReconciler{
		Client:          r.Client,
		Log:             r.Log,
		Recorder:        r.Recorder,
		ServiceEndpoint: r.ServiceEndpoint,
		Scheme:          r.Scheme,
	}).SetupWithManager(ctx, mgr)
}

// IBMVPCMachineTemplateReconciler reconciles a IBMVPCMachineTemplate object.
type IBMVPCMachineTemplateReconciler struct {
	client.Client
	Scheme          *runtime.Scheme
	ServiceEndpoint []endpoints.ServiceEndpoint
}

func (r *IBMVPCMachineTemplateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return (&vpccontroller.IBMVPCMachineTemplateReconciler{
		Client:          r.Client,
		Scheme:          r.Scheme,
		ServiceEndpoint: r.ServiceEndpoint,
	}).SetupWithManager(ctx, mgr)
}

// IBMPowerVSClusterReconciler reconciles a IBMPowerVSCluster object.
type IBMPowerVSClusterReconciler struct {
	client.Client
	Recorder        record.EventRecorder
	ServiceEndpoint []endpoints.ServiceEndpoint
	Scheme          *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

func (r *IBMPowerVSClusterReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return (&powervscontroller.IBMPowerVSClusterReconciler{
		Client:           r.Client,
		Recorder:         r.Recorder,
		ServiceEndpoint:  r.ServiceEndpoint,
		Scheme:           r.Scheme,
		WatchFilterValue: r.WatchFilterValue,
	}).SetupWithManager(ctx, mgr)
}

// IBMPowerVSMachineReconciler reconciles a IBMPowerVSMachine object.
type IBMPowerVSMachineReconciler struct {
	client.Client
	Recorder        record.EventRecorder
	ServiceEndpoint []endpoints.ServiceEndpoint
	Scheme          *runtime.Scheme

	// WatchFilterValue is the label value used to filter events prior to reconciliation.
	WatchFilterValue string
}

func (r *IBMPowerVSMachineReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return (&powervscontroller.IBMPowerVSMachineReconciler{
		Client:           r.Client,
		Recorder:         r.Recorder,
		ServiceEndpoint:  r.ServiceEndpoint,
		Scheme:           r.Scheme,
		WatchFilterValue: r.WatchFilterValue,
	}).SetupWithManager(ctx, mgr)
}

// IBMPowerVSMachineTemplateReconciler reconciles a IBMPowerVSMachineTemplate object.
type IBMPowerVSMachineTemplateReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func (r *IBMPowerVSMachineTemplateReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return (&powervscontroller.IBMPowerVSMachineTemplateReconciler{
		Client: r.Client,
		Scheme: r.Scheme,
	}).SetupWithManager(ctx, mgr)
}

// IBMPowerVSImageReconciler reconciles a IBMPowerVSImage object.
type IBMPowerVSImageReconciler struct {
	client.Client
	Recorder        record.EventRecorder
	ServiceEndpoint []endpoints.ServiceEndpoint
	Scheme          *runtime.Scheme
}

func (r *IBMPowerVSImageReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return (&powervscontroller.IBMPowerVSImageReconciler{
		Client:          r.Client,
		Recorder:        r.Recorder,
		ServiceEndpoint: r.ServiceEndpoint,
		Scheme:          r.Scheme,
	}).SetupWithManager(ctx, mgr)
}
