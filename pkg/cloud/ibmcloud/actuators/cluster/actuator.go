/*
Copyright 2019 The Kubernetes authors.

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
	"context"
	"fmt"

	"github.com/pkg/errors"
	"k8s.io/klog"

	providerv1 "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/apis/ibmcloud/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/ibmcloud"
	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type IBMCloudClient struct {
	params ibmcloud.ActuatorParams
	client client.Client
}

// NewActuator creates a new Actuator
// NewActuator creates a new Actuator
func NewActuator(params ibmcloud.ActuatorParams) (*IBMCloudClient, error) {
	return &IBMCloudClient{
		params: params,
		client: params.Client,
	}, nil
}

// Reconcile reconciles a cluster and is invoked by the Cluster Controller
func (a *IBMCloudClient) Reconcile(cluster *clusterv1.Cluster) error {
	if cluster == nil {
		return fmt.Errorf("The cluster is nil, check your cluster configuration")
	}

	klog.Infof("Reconciling cluster %v.", cluster.Name)

	status, err := providerv1.ClusterStatusFromProviderStatus(cluster.Status.ProviderStatus)
	if err != nil {
		return errors.Errorf("failed to load cluster provider status: %v", err)
	}

	defer func() {
		if err := a.storeClusterStatus(cluster, status); err != nil {
			klog.Errorf("failed to store provider status for cluster %q in namespace %q: %v", cluster.Name, cluster.Namespace, err)
		}
	}()
	return nil
}

func (a *IBMCloudClient) storeClusterStatus(cluster *clusterv1.Cluster, status *providerv1.IBMCloudClusterProviderStatus) error {
	ext, err := providerv1.EncodeClusterStatus(status)
	if err != nil {
		return fmt.Errorf("failed to update cluster status for cluster %q in namespace %q: %v", cluster.Name, cluster.Namespace, err)
	}
	cluster.Status.ProviderStatus = ext

	statusClient := a.params.Client.Status()
	if err := statusClient.Update(context.TODO(), cluster); err != nil {
		return fmt.Errorf("failed to update cluster status for cluster %q in namespace %q: %v", cluster.Name, cluster.Namespace, err)
	}

	return nil
}

// Delete deletes a cluster and is invoked by the Cluster Controller
func (a *IBMCloudClient) Delete(cluster *clusterv1.Cluster) error {
	klog.Infof("Deleting cluster %v.", cluster.Name)

	klog.Infof("Deleting cluster %v: Not implemented yet.", cluster.Name)

	return nil
}
