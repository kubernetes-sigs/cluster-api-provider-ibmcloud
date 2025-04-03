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

package scope

import (
	"fmt"
	"os"
	"path"
	"testing"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	ctrl "sigs.k8s.io/controller-runtime"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/internal/webhooks"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/test/helpers"
)

var (
	testEnv *helpers.TestEnvironment
	ctx     = ctrl.SetupSignalHandler()
)

func TestMain(m *testing.M) {
	setup()
	result := m.Run()
	teardown()
	os.Exit(result)
}

// Setting up the test environment.
func setup() {
	utilruntime.Must(infrav1beta2.AddToScheme(scheme.Scheme))
	utilruntime.Must(capiv1beta1.AddToScheme(scheme.Scheme))
	testEnvConfig := helpers.NewTestEnvironmentConfiguration([]string{
		path.Join("config", "crd", "bases"),
	},
	).WithWebhookConfiguration("unmanaged", path.Join("config", "webhook", "manifests.yaml"))
	var err error
	testEnv, err = testEnvConfig.Build()
	if err != nil {
		panic(err)
	}
	if err := (&webhooks.IBMPowerVSCluster{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMPowerVSCluster webhook: %v", err))
	}
	if err := (&webhooks.IBMPowerVSMachine{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMPowerVSMachine webhook: %v", err))
	}
	if err := (&webhooks.IBMPowerVSMachineTemplate{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMPowerVSMachineTemplate webhook: %v", err))
	}
	if err := (&webhooks.IBMPowerVSImage{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMPowerVSImage webhook: %v", err))
	}
	if err := (&webhooks.IBMVPCCluster{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMVPCCluster webhook: %v", err))
	}
	if err := (&webhooks.IBMVPCMachine{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMVPCMachine webhook: %v", err))
	}
	if err := (&webhooks.IBMVPCMachineTemplate{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMVPCMachineTemplate webhook: %v", err))
	}
	if err := (&webhooks.IBMPowerVSClusterTemplate{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMPowerVSClusterTemplate webhook: %v", err))
	}
	go func() {
		if err := testEnv.StartManager(ctx); err != nil {
			panic(fmt.Sprintf("Failed to start the envtest manager: %v", err))
		}
	}()
	testEnv.WaitForWebhooks()
}

func teardown() {
	if err := testEnv.Stop(); err != nil {
		panic(fmt.Sprintf("Failed to stop envtest: %v", err))
	}
}
