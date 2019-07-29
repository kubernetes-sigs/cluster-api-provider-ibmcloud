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

package cluster

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	providerv1 "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/apis/ibmcloud/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/ibmcloud"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	yaml "sigs.k8s.io/yaml"
)

func TestDelete(t *testing.T) {
	params := ibmcloud.ActuatorParams{
		Client:     nil,
		KubeClient: nil,
	}

	ibmcloudclient, err := NewActuator(params)
	if err != nil {
		t.Errorf("Could not create actuator: %v", err)
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testCluster",
			Namespace: "testNamespace",
		},
	}

	err = ibmcloudclient.Delete(cluster)

	if err != nil {
		t.Errorf("Failed to delete cluster: %v", err)
	}

}

func TestReconcile(t *testing.T) {
	client := fake.NewFakeClient()

	params := ibmcloud.ActuatorParams{
		Client:     client,
		KubeClient: nil,
	}

	ibmcloudclient, err := NewActuator(params)
	if err != nil {
		t.Errorf("Could not create actuator: %v", err)
	}

	providerStatus := &providerv1.IBMCloudClusterProviderStatus{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "foo",
			Namespace: "default",
		},
	}

	bytes, err := yaml.Marshal(providerStatus)

	if err != nil {
		t.Error(err)
	}

	cluster := &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testCluster",
			Namespace: "testNamespace",
		},
		Status: clusterv1.ClusterStatus{
			ProviderStatus: &runtime.RawExtension{
				Raw: bytes,
			},
		},
	}

	err = ibmcloudclient.Reconcile(cluster)

	if err != nil {
		t.Errorf("Failed to reconcile cluster: %v", err)
	}
}
