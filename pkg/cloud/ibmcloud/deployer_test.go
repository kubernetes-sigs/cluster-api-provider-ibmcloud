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

package ibmcloud

import (
	"os"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	ibmcloudv1 "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/apis/ibmcloud/v1alpha1"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	yaml "sigs.k8s.io/yaml"
)

func TestGetIP(t *testing.T) {
	// exist IP case
	testIP := "10.0.0.1"

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testCluster",
			Namespace: "testNamespace",
		},
	}

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testMachine",
			Namespace: "testNamespace",
			Annotations: map[string]string{
				IBMCloudIPAnnotationKey: testIP,
			},
		},
	}

	deploy := NewDeploymentClient()
	ip, err := deploy.GetIP(cluster, machine)
	if err != nil || ip != testIP {
		t.Errorf("Unable to get right machine IP %s", testIP)
	}
}

func TestGetKubeConfig(t *testing.T) {
	// test IP
	testIP := "127.0.0.1"

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testCluster",
			Namespace: "testNamespace",
		},
	}
	// ssh user name
	tmpConfig := ibmcloudv1.IBMCloudMachineProviderSpec{
		SshUserName: "testUser",
	}

	bytes, err := yaml.Marshal(tmpConfig)

	if err != nil {
		t.Error(err)
	}

	machine := &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testMachine",
			Namespace: "testNamespace",
			Annotations: map[string]string{
				IBMCloudIPAnnotationKey: testIP,
			},
		},
		Spec: clusterv1.MachineSpec{
			ProviderSpec: clusterv1.ProviderSpec{
				Value: &runtime.RawExtension{
					Raw: bytes,
				},
			},
		},
	}

	// test providerSpec
	_, err = ibmcloudv1.MachineSpecFromProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		t.Error(err)
	}

	deploy := NewDeploymentClient()
	config, err := deploy.GetKubeConfig(cluster, machine)
	if err != nil {
		t.Error(err)
	}
	if config != "" {
		t.Logf("Found kubeconfig: %s", config)
	}

}

func TestGetSSHKeyFile(t *testing.T) {
	// default case
	homeDir, ok := os.LookupEnv("HOME")
	if !ok {
		t.Errorf("Unable to use HOME environment variable to find SSH key")
	}
	defaultKey := homeDir + "/.ssh/id_ibmcloud"
	targetKeyfile := getSSHKeyFile(homeDir)
	if 0 != strings.Compare(targetKeyfile, defaultKey) {
		t.Errorf("Unexpected output: %s, expect output: %s", targetKeyfile, defaultKey)
	}

	// custom case
	customKey := "examples/ibmcloud/mykey"
	err := os.Setenv("IBMCLOUD_HOST_SSH_PRIVATE_FILE", customKey)
	if err != nil {
		t.Errorf("Can not set environment variable IBMCLOUD_HOST_SSH_PRIVATE_FILE")
	}
	targetKeyfile = getSSHKeyFile(homeDir)
	if 0 != strings.Compare(targetKeyfile, customKey) {
		t.Errorf("Unexpected output: %s, expect output: %s", targetKeyfile, customKey)
	}
}
