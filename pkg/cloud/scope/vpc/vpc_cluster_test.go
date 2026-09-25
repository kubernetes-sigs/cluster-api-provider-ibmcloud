/*
Copyright 2024 The Kubernetes Authors.

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

package vpc

import (
	"context"
	"testing"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"go.uber.org/mock/gomock"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/vpc/v1beta2"
	mockgt "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/globaltagging/mock"
	mockrm "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcemanager/mock"
	mockvpc "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

// setupClusterScopeV2 builds a ClusterScopeV2 directly (no NewClusterScopeV2, no envtest)
// with the provided mocks injected. ResourceGroup ID is pre-populated in Status so that
// createLoadBalancer can skip the ResourceManager API call unless the test needs to test that path.
func setupClusterScopeV2(
	t *testing.T,
	vpcCluster *infrav1.IBMVPCCluster,
	mockVPC *mockvpc.MockVpc,
	mockRM *mockrm.MockResourceManager,
	mockGT *mockgt.MockGlobalTagging,
) *ClusterScopeV2 {
	t.Helper()
	cluster := newCluster(clusterName)
	initObjects := []client.Object{cluster, vpcCluster}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
	return &ClusterScopeV2{
		Logger:                klog.Background(),
		Client:                fakeClient,
		Cluster:               cluster,
		IBMVPCCluster:         vpcCluster,
		VPCClient:             mockVPC,
		ResourceManagerClient: mockRM,
		GlobalTaggingClient:   mockGT,
	}
}

func newVPCClusterWithNetwork(name string, lbs []infrav1.VPCLoadBalancerSpec) *infrav1.IBMVPCCluster {
	c := newVPCCluster(name)
	c.Spec.Network = &infrav1.VPCNetworkSpec{
		LoadBalancers: lbs,
	}
	// Pre-populate ResourceGroup in Status so GetResourceGroupID() returns without an API call.
	// Also initialise Network status so callers can safely set ControlPlaneSubnets.
	c.Status.ResourceGroup = &infrav1.ResourceStatus{
		ID: "test-resource-group-id",
	}
	c.Status.Network = &infrav1.VPCNetworkStatus{}
	return c
}

func TestVPCClusterReconcileLoadBalancers(t *testing.T) {
	ctx := context.Background()

	setup := func(t *testing.T) (*gomock.Controller, *mockvpc.MockVpc, *mockrm.MockResourceManager, *mockgt.MockGlobalTagging) {
		t.Helper()
		mc := gomock.NewController(t)
		return mc, mockvpc.NewMockVpc(mc), mockrm.NewMockResourceManager(mc), mockgt.NewMockGlobalTagging(mc)
	}

	t.Run("Error when no load balancers defined", func(t *testing.T) {
		g := NewWithT(t)
		mc, mockVPC, mockRM, mockGT := setup(t)
		t.Cleanup(mc.Finish)

		vpcCluster := newVPCCluster(clusterName)
		vpcCluster.Spec.Network = &infrav1.VPCNetworkSpec{}
		scope := setupClusterScopeV2(t, vpcCluster, mockVPC, mockRM, mockGT)

		_, err := scope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("no load balancers specified"))
	})

	t.Run("Error when more than two load balancers defined", func(t *testing.T) {
		g := NewWithT(t)
		mc, mockVPC, mockRM, mockGT := setup(t)
		t.Cleanup(mc.Finish)

		lbs := []infrav1.VPCLoadBalancerSpec{
			{Name: "lb-1"},
			{Name: "lb-2"},
			{Name: "lb-3"},
		}
		vpcCluster := newVPCClusterWithNetwork(clusterName, lbs)
		scope := setupClusterScopeV2(t, vpcCluster, mockVPC, mockRM, mockGT)

		_, err := scope.ReconcileLoadBalancers(ctx)
		g.Expect(err).To(HaveOccurred())
		g.Expect(err.Error()).To(ContainSubstring("maximum of two load balancers"))
	})

	t.Run("LB already exists and is active — no requeue", func(t *testing.T) {
		g := NewWithT(t)
		mc, mockVPC, mockRM, mockGT := setup(t)
		t.Cleanup(mc.Finish)

		lbID := "existing-lb-id"
		lbs := []infrav1.VPCLoadBalancerSpec{
			{Name: "my-lb"},
		}
		vpcCluster := newVPCClusterWithNetwork(clusterName, lbs)
		scope := setupClusterScopeV2(t, vpcCluster, mockVPC, mockRM, mockGT)

		mockVPC.EXPECT().GetLoadBalancerByName("my-lb").Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateActive)),
			Hostname:           ptr.To("my-lb.example.com"),
		}, nil)

		requeue, err := scope.ReconcileLoadBalancers(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(requeue).To(BeFalse())
	})

	t.Run("LB already exists but not active — requeue", func(t *testing.T) {
		g := NewWithT(t)
		mc, mockVPC, mockRM, mockGT := setup(t)
		t.Cleanup(mc.Finish)

		lbID := "existing-lb-id"
		lbs := []infrav1.VPCLoadBalancerSpec{
			{Name: "my-lb"},
		}
		vpcCluster := newVPCClusterWithNetwork(clusterName, lbs)
		scope := setupClusterScopeV2(t, vpcCluster, mockVPC, mockRM, mockGT)

		mockVPC.EXPECT().GetLoadBalancerByName("my-lb").Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateCreatePending)),
			Hostname:           ptr.To("my-lb.example.com"),
		}, nil)

		requeue, err := scope.ReconcileLoadBalancers(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(requeue).To(BeTrue())
	})
}
