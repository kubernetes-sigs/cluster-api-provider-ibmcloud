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

package scope

import (
	"context"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/globaltaggingv1"
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

// setupVPCClusterScope builds a VPCClusterScope directly (no NewVPCClusterScope, no envtest)
// with the provided mocks injected. ResourceGroup ID is pre-populated in Status so that
// createLoadBalancer can skip the ResourceManager API call unless the test needs to test that path.
func setupVPCClusterScope(
	t *testing.T,
	vpcCluster *infrav1.IBMVPCCluster,
	mockVPC *mockvpc.MockVpc,
	mockRM *mockrm.MockResourceManager,
	mockGT *mockgt.MockGlobalTagging,
) *VPCClusterScope {
	t.Helper()
	cluster := newCluster(clusterName)
	initObjects := []client.Object{cluster, vpcCluster}
	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(initObjects...).Build()
	return &VPCClusterScope{
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
		scope := setupVPCClusterScope(t, vpcCluster, mockVPC, mockRM, mockGT)

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
		scope := setupVPCClusterScope(t, vpcCluster, mockVPC, mockRM, mockGT)

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
		scope := setupVPCClusterScope(t, vpcCluster, mockVPC, mockRM, mockGT)

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
		scope := setupVPCClusterScope(t, vpcCluster, mockVPC, mockRM, mockGT)

		mockVPC.EXPECT().GetLoadBalancerByName("my-lb").Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To(lbID),
			ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateCreatePending)),
			Hostname:           ptr.To("my-lb.example.com"),
		}, nil)

		requeue, err := scope.ReconcileLoadBalancers(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(requeue).To(BeTrue())
	})

	t.Run("Creates application LB (default profile) when not found", func(t *testing.T) {
		g := NewWithT(t)
		mc, mockVPC, mockRM, mockGT := setup(t)
		t.Cleanup(mc.Finish)

		appProfile := infrav1.VPCLoadBalancerProfileApplication
		lbs := []infrav1.VPCLoadBalancerSpec{
			{
				Name:    "my-app-lb",
				Public:  ptr.To(true),
				Profile: &appProfile,
			},
		}
		vpcCluster := newVPCClusterWithNetwork(clusterName, lbs)
		// Pre-populate a control-plane subnet in Status for LB subnet selection.
		vpcCluster.Status.Network.ControlPlaneSubnets = map[string]*infrav1.ResourceStatus{
			"subnet-a": {ID: "subnet-id-a"},
		}
		scope := setupVPCClusterScope(t, vpcCluster, mockVPC, mockRM, mockGT)

		// LB doesn't exist yet.
		mockVPC.EXPECT().GetLoadBalancerByName("my-app-lb").Return(nil, nil)
		// Expect create with application profile.
		mockVPC.EXPECT().CreateLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerOptions{})).
			DoAndReturn(func(opts *vpcv1.CreateLoadBalancerOptions) (*vpcv1.LoadBalancer, *core.DetailedResponse, error) {
				g.Expect(opts.Name).To(Equal(ptr.To("my-app-lb")))
				g.Expect(opts.IsPublic).To(Equal(ptr.To(true)))
				g.Expect(opts.Profile).NotTo(BeNil())
				profile, ok := opts.Profile.(*vpcv1.LoadBalancerProfileIdentityByName)
				g.Expect(ok).To(BeTrue(), "profile should be LoadBalancerProfileIdentityByName")
				g.Expect(profile.Name).To(Equal(ptr.To("application")))
				return &vpcv1.LoadBalancer{
					ID:                 ptr.To("new-lb-id"),
					CRN:                ptr.To("crn:new-lb"),
					ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateCreatePending)),
					Hostname:           ptr.To("my-app-lb.example.com"),
				}, &core.DetailedResponse{}, nil
			})
		// TagResource: tag already exists.
		mockGT.EXPECT().GetTagByName(gomock.Any()).Return(&globaltaggingv1.Tag{Name: ptr.To(clusterName)}, nil)
		mockGT.EXPECT().AttachTag(gomock.AssignableToTypeOf(&globaltaggingv1.AttachTagOptions{})).Return(&globaltaggingv1.TagResults{}, &core.DetailedResponse{}, nil)

		requeue, err := scope.ReconcileLoadBalancers(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(requeue).To(BeTrue(), "newly created LB should trigger requeue")
	})

	t.Run("Creates network-fixed NLB when profile is network-fixed", func(t *testing.T) {
		g := NewWithT(t)
		mc, mockVPC, mockRM, mockGT := setup(t)
		t.Cleanup(mc.Finish)

		nlbProfile := infrav1.VPCLoadBalancerProfileNetworkFixed
		lbs := []infrav1.VPCLoadBalancerSpec{
			{
				Name:    "my-nlb",
				Public:  ptr.To(true),
				Profile: &nlbProfile,
			},
		}
		vpcCluster := newVPCClusterWithNetwork(clusterName, lbs)
		vpcCluster.Status.Network.ControlPlaneSubnets = map[string]*infrav1.ResourceStatus{
			"subnet-a": {ID: "subnet-id-a"},
		}
		scope := setupVPCClusterScope(t, vpcCluster, mockVPC, mockRM, mockGT)

		// LB doesn't exist yet.
		mockVPC.EXPECT().GetLoadBalancerByName("my-nlb").Return(nil, nil)
		// Expect create with network-fixed profile — this is the key assertion.
		mockVPC.EXPECT().CreateLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerOptions{})).
			DoAndReturn(func(opts *vpcv1.CreateLoadBalancerOptions) (*vpcv1.LoadBalancer, *core.DetailedResponse, error) {
				g.Expect(opts.Name).To(Equal(ptr.To("my-nlb")))
				g.Expect(opts.IsPublic).To(Equal(ptr.To(true)))
				g.Expect(opts.Profile).NotTo(BeNil(), "profile must be set for network-fixed NLB")
				profile, ok := opts.Profile.(*vpcv1.LoadBalancerProfileIdentityByName)
				g.Expect(ok).To(BeTrue(), "profile should be LoadBalancerProfileIdentityByName")
				g.Expect(profile.Name).To(Equal(ptr.To("network-fixed")), "profile name must be network-fixed")
				return &vpcv1.LoadBalancer{
					ID:                 ptr.To("new-nlb-id"),
					CRN:                ptr.To("crn:new-nlb"),
					ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateCreatePending)),
					Hostname:           ptr.To("my-nlb.example.com"),
				}, &core.DetailedResponse{}, nil
			})
		// TagResource: tag already exists.
		mockGT.EXPECT().GetTagByName(gomock.Any()).Return(&globaltaggingv1.Tag{Name: ptr.To(clusterName)}, nil)
		mockGT.EXPECT().AttachTag(gomock.AssignableToTypeOf(&globaltaggingv1.AttachTagOptions{})).Return(&globaltaggingv1.TagResults{}, &core.DetailedResponse{}, nil)

		requeue, err := scope.ReconcileLoadBalancers(ctx)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(requeue).To(BeTrue(), "newly created NLB should trigger requeue")

		// Verify LB status was populated.
		g.Expect(scope.NetworkStatus()).NotTo(BeNil())
		g.Expect(scope.NetworkStatus().LoadBalancers).To(HaveKey("new-nlb-id"))
		lbStatus := scope.NetworkStatus().LoadBalancers["new-nlb-id"]
		g.Expect(lbStatus.State).To(Equal(infrav1.VPCLoadBalancerStateCreatePending))
		g.Expect(*lbStatus.Hostname).To(Equal("my-nlb.example.com"))
	})

	t.Run("No profile set — profile field omitted in API call", func(t *testing.T) {
		g := NewWithT(t)
		mc, mockVPC, mockRM, mockGT := setup(t)
		t.Cleanup(mc.Finish)

		lbs := []infrav1.VPCLoadBalancerSpec{
			{
				Name:   "my-default-lb",
				Public: ptr.To(true),
				// Profile deliberately omitted.
			},
		}
		vpcCluster := newVPCClusterWithNetwork(clusterName, lbs)
		vpcCluster.Status.Network.ControlPlaneSubnets = map[string]*infrav1.ResourceStatus{
			"subnet-a": {ID: "subnet-id-a"},
		}
		scope := setupVPCClusterScope(t, vpcCluster, mockVPC, mockRM, mockGT)

		mockVPC.EXPECT().GetLoadBalancerByName("my-default-lb").Return(nil, nil)
		mockVPC.EXPECT().CreateLoadBalancer(gomock.AssignableToTypeOf(&vpcv1.CreateLoadBalancerOptions{})).
			DoAndReturn(func(opts *vpcv1.CreateLoadBalancerOptions) (*vpcv1.LoadBalancer, *core.DetailedResponse, error) {
				g.Expect(opts.Profile).To(BeNil(), "profile field must not be set when Profile is nil in spec")
				return &vpcv1.LoadBalancer{
					ID:                 ptr.To("new-lb-id"),
					CRN:                ptr.To("crn:new-lb"),
					ProvisioningStatus: ptr.To(string(infrav1.VPCLoadBalancerStateCreatePending)),
					Hostname:           ptr.To("my-default-lb.example.com"),
				}, &core.DetailedResponse{}, nil
			})
		mockGT.EXPECT().GetTagByName(gomock.Any()).Return(&globaltaggingv1.Tag{Name: ptr.To(clusterName)}, nil)
		mockGT.EXPECT().AttachTag(gomock.AssignableToTypeOf(&globaltaggingv1.AttachTagOptions{})).Return(&globaltaggingv1.TagResults{}, &core.DetailedResponse{}, nil)

		_, err := scope.ReconcileLoadBalancers(ctx)
		g.Expect(err).NotTo(HaveOccurred())
	})
}
