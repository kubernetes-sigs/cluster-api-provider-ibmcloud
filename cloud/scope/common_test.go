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
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
)

const (
	clusterName = "foo-cluster"
	machineName = "foo-machine"
	pvsImage    = "foo-image"
	pvsNetwork  = "foo-network"
)

func newCluster(name string) *clusterv1.Cluster {
	return &clusterv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: clusterv1.ClusterSpec{},
	}
}

func newVPCCluster(name string) *infrav1.IBMVPCCluster {
	return &infrav1.IBMVPCCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
}

func newPowerVSCluster(name string) *infrav1.IBMPowerVSCluster {
	return &infrav1.IBMPowerVSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
	}
}

func newMachine(machineName string) *clusterv1.Machine {
	return &clusterv1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name:      machineName,
			Namespace: "default",
		},
		Spec: clusterv1.MachineSpec{
			Bootstrap: clusterv1.Bootstrap{
				DataSecretName: core.StringPtr(machineName),
			},
		},
	}
}

func newBootstrapSecret(clusterName, machineName string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				clusterv1.ClusterNameLabel: clusterName,
			},
			Name:      machineName,
			Namespace: "default",
		},
		Data: map[string][]byte{
			"value": []byte("user data"),
		},
	}
}

func newDHCPServerDetails(serverID, leaseIP, instanceMac string) *models.DHCPServerDetail {
	return &models.DHCPServerDetail{
		ID: ptr.To(serverID),
		Leases: []*models.DHCPServerLeases{
			{
				InstanceIP:         ptr.To(leaseIP),
				InstanceMacAddress: ptr.To(instanceMac),
			},
		},
	}
}
