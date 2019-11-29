/*
Copyright 2019 The Kubernetes Authors.

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

	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/runtime/signals"

	clusterapis "sigs.k8s.io/cluster-api/pkg/apis"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/apis"
	_ "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/ibmcloud/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/controller"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/record"
)

func main() {
	klog.InitFlags(nil)
	flag.Parse()

	klog.Info("Starting controller of IBM cloud provider for cluster api")

	cfg := config.GetConfigOrDie()
	if cfg == nil {
		panic(fmt.Errorf("GetConfigOrDie didn't die"))
	}

	// Setup a Manager
	mgr, err := manager.New(cfg, manager.Options{
		LeaderElectionID: "controller-leader-election-cluster-api-provider-ibmcloud",
	})
	if err != nil {
		klog.Fatalf("unable to set up overall controller manager: %v", err)
	}

	record.InitFromRecorder(mgr.GetRecorder("ibmcloud-controller"))

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Fatalf("Error adding apis scheme: %v", err)
	}

	if err := clusterapis.AddToScheme(mgr.GetScheme()); err != nil {
		klog.Fatalf("Error adding cluster apis scheme: %v", err)
	}

	if err := controller.AddToManager(mgr); err != nil {
		klog.Fatalf("Error initializing controllers: %v", err)
	}

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		klog.Fatalf("Failed starting controller: %v", err)
	}
}
