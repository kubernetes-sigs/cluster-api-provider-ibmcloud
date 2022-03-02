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

package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"

	infrastructurev1alpha3 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1alpha3"
	infrastructurev1alpha4 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1alpha4"
	infrastructurev1beta1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/controllers"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/options"
	// +kubebuilder:scaffold:imports
)

var (
	watchNamespace       string
	metricsAddr          string
	enableLeaderElection bool
	healthAddr           string
	syncPeriod           time.Duration

	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	_ = clientgoscheme.AddToScheme(scheme)

	_ = infrastructurev1alpha3.AddToScheme(scheme)
	_ = infrastructurev1alpha4.AddToScheme(scheme)
	_ = infrastructurev1beta1.AddToScheme(scheme)
	_ = clusterv1.AddToScheme(scheme)
	// +kubebuilder:scaffold:scheme
}

func main() {
	klog.InitFlags(nil)

	initFlags(pflag.CommandLine)
	pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	pflag.Parse()

	ctrl.SetLogger(klogr.New())

	if err := validateFlags(); err != nil {
		setupLog.Error(err, "Flag validation failure")
		os.Exit(1)
	}

	if watchNamespace != "" {
		setupLog.Info("Watching cluster-api objects only in namespace for reconciliation", "namespace", watchNamespace)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		MetricsBindAddress:     metricsAddr,
		Port:                   9443,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "effcf9b8.cluster.x-k8s.io",
		SyncPeriod:             &syncPeriod,
		Namespace:              watchNamespace,
		HealthProbeBindAddress: healthAddr,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.IBMVPCClusterReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("IBMVPCCluster"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IBMVPCCluster")
		os.Exit(1)
	}
	if err = (&controllers.IBMVPCMachineReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("IBMVPCMachine"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IBMVPCMachine")
		os.Exit(1)
	}
	if err = (&controllers.IBMPowerVSClusterReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("IBMPowerVSCluster"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IBMPowerVSCluster")
		os.Exit(1)
	}
	if err = (&controllers.IBMPowerVSMachineReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("IBMPowerVSMachine"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IBMPowerVSMachine")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.IBMPowerVSCluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "IBMPowerVSCluster")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.IBMPowerVSMachine{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "IBMPowerVSMachine")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.IBMPowerVSMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "IBMPowerVSMachineTemplate")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.IBMVPCMachine{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "IBMVPCMachine")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.IBMVPCMachineTemplate{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "IBMVPCMachineTemplate")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.IBMVPCCluster{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "IBMVPCCluster")
		os.Exit(1)
	}
	if err = (&controllers.IBMPowerVSImageReconciler{
		Client: mgr.GetClient(),
		Log:    ctrl.Log.WithName("controllers").WithName("IBMPowerVSImage"),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "IBMPowerVSImage")
		os.Exit(1)
	}
	if err = (&infrastructurev1beta1.IBMPowerVSImage{}).SetupWebhookWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create webhook", "webhook", "IBMPowerVSImage")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if err := mgr.AddReadyzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create ready check")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("webhook", mgr.GetWebhookServer().StartedChecker()); err != nil {
		setupLog.Error(err, "unable to create health check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func initFlags(fs *pflag.FlagSet) {
	fs.StringVar(
		&metricsAddr,
		"metrics-bind-addr",
		":8080",
		"The address the metric endpoint binds to.",
	)

	fs.BoolVar(
		&enableLeaderElection,
		"leader-elect",
		false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.",
	)

	fs.StringVar(
		&watchNamespace,
		"namespace",
		"",
		"Namespace that the controller watches to reconcile cluster-api objects. If unspecified, the controller watches for cluster-api objects across all namespaces.",
	)

	fs.StringVar(
		&healthAddr,
		"health-addr",
		":9440",
		"The address the health endpoint binds to.",
	)

	fs.DurationVar(
		&syncPeriod,
		"sync-period",
		10*time.Minute,
		"The minimum interval at which watched resources are reconciled.",
	)

	fs.StringVar(
		&options.PowerVSProviderIDFormat,
		"powervs-provider-id-fmt",
		options.PowerVSProviderIDFormatV1,
		"ProviderID format is used set the Provider ID format for Machine",
	)
}

func validateFlags() error {
	switch options.PowerVSProviderIDFormat {
	case options.PowerVSProviderIDFormatV1:
		setupLog.Info("Using v1 version of ProviderID format")
	case options.PowerVSProviderIDFormatV2:
		setupLog.Info("Using v2 version of ProviderID format")
	default:
		errStr := fmt.Errorf("Invalid value for flag powervs-provider-id-fmt: %s, Supported values: v1, v2 ", options.PowerVSProviderIDFormat)
		return errStr
	}
	return nil
}
