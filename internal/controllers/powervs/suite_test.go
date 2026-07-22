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

package powervs

import (
	"context"
	"fmt"
	"os"
	"path"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"

	// +kubebuilder:scaffold:imports
	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	webhookspowervs "sigs.k8s.io/cluster-api-provider-ibmcloud/internal/webhooks/powervs"
	powervsscope "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/scope/powervs"
	powervssvc "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcemanager"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/transitgateway"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/test/helpers"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
)

// stubClientBuilder is a test-only ClientBuilder that returns nil clients for tests
// that inject mocks directly into scope fields.
type stubClientBuilder struct{}

func (s stubClientBuilder) GetAuthenticator(_ context.Context) (core.Authenticator, error) {
	return nil, nil
}
func (s stubClientBuilder) GetPowerVSClient(_ context.Context, _ powervsscope.ClientOptions) (powervssvc.PowerVS, error) {
	return nil, nil
}
func (s stubClientBuilder) GetVPCClient(_ context.Context, _ powervsscope.ClientOptions) (vpc.Vpc, error) {
	return nil, nil
}
func (s stubClientBuilder) GetTransitGatewayClient(_ context.Context, _ powervsscope.ClientOptions) (transitgateway.TransitGateway, error) {
	return nil, nil
}
func (s stubClientBuilder) GetResourceControllerClient(_ context.Context, _ powervsscope.ClientOptions) (resourcecontroller.ResourceController, error) {
	return nil, nil
}
func (s stubClientBuilder) GetResourceManagerClient(_ context.Context, _ powervsscope.ClientOptions) (resourcemanager.ResourceManager, error) {
	return nil, nil
}

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
	utilruntime.Must(infrav1.AddToScheme(scheme.Scheme))
	utilruntime.Must(clusterv1.AddToScheme(scheme.Scheme))
	testEnvConfig := helpers.NewTestEnvironmentConfiguration([]string{
		path.Join("config", "crd", "bases"),
	},
	).WithWebhookConfiguration("unmanaged", path.Join("config", "webhook", "manifests.yaml"))
	var err error
	testEnv, err = testEnvConfig.Build()
	if err != nil {
		panic(err)
	}
	if err := (&webhookspowervs.IBMPowerVSCluster{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMPowerVSCluster webhook: %v", err))
	}
	if err := (&webhookspowervs.IBMPowerVSClusterTemplate{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMPowerVSClusterTemplate webhook: %v", err))
	}
	if err := (&webhookspowervs.IBMPowerVSMachine{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMPowerVSMachine webhook: %v", err))
	}
	if err := (&webhookspowervs.IBMPowerVSMachineTemplate{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMPowerVSMachineTemplate webhook: %v", err))
	}
	if err := (&webhookspowervs.IBMPowerVSImage{}).SetupWebhookWithManager(testEnv); err != nil {
		panic(fmt.Sprintf("Unable to setup IBMPowerVSImage webhook: %v", err))
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
