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
	"errors"
	"testing"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	. "github.com/onsi/gomega"
	"go.uber.org/mock/gomock"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/vpc/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"
)

// setupClusterScopeV2WithMock creates a minimal ClusterScopeV2 with a mock VPC client
// and the given IBMVPCCluster, ready for routing table testing.
func setupClusterScopeV2WithMock(ibmvpcCluster *infrav1.IBMVPCCluster, mockvpc *mock.MockVpc) *ClusterScopeV2 {
	return &ClusterScopeV2{
		IBMVPCCluster: ibmvpcCluster,
		VPCClient:     mockvpc,
	}
}

// newIBMVPCClusterWithNetwork returns a minimal IBMVPCCluster with the given network spec.
func newIBMVPCClusterWithNetwork(name string, networkSpec *infrav1.VPCNetworkSpec) *infrav1.IBMVPCCluster {
	return &infrav1.IBMVPCCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
		},
		Spec: infrav1.IBMVPCClusterSpec{
			Region:        "us-south",
			ResourceGroup: "default-rg",
			Network:       networkSpec,
		},
	}
}

// routingTableStable returns a RoutingTable with stable lifecycle state.
func routingTableStable(id, name string) *vpcv1.RoutingTable {
	stable := string(vpcv1.RoutingTableLifecycleStateStableConst)
	return &vpcv1.RoutingTable{
		ID:             ptr.To(id),
		Name:           ptr.To(name),
		CRN:            ptr.To("crn:v1:test::" + id),
		LifecycleState: &stable,
	}
}

// routingTablePending returns a RoutingTable with pending lifecycle state.
func routingTablePending(id, name string) *vpcv1.RoutingTable {
	pending := "pending"
	return &vpcv1.RoutingTable{
		ID:             ptr.To(id),
		Name:           ptr.To(name),
		CRN:            ptr.To("crn:v1:test::" + id),
		LifecycleState: &pending,
	}
}

func TestReconcileRoutingTables_NoRoutingTables(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		VPC: &infrav1.VPCResource{ID: ptr.To("vpc-id")},
		// no routing tables
	})
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	requeue, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).To(BeNil())
	g.Expect(requeue).To(BeFalse())
}

func TestReconcileRoutingTables_RoutingTableNoIDOrName(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		VPC: &infrav1.VPCResource{ID: ptr.To("vpc-id")},
		RoutingTables: []infrav1.VPCRoutingTable{
			{
				// No ID and no Name — invalid
			},
		},
	})
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	_, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).NotTo(BeNil())
	g.Expect(err.Error()).To(ContainSubstring("no id or name"))
}

func TestReconcileRoutingTables_FoundByID_Stable(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	rtID := "rt-abc123"
	rtName := "my-routing-table"
	vpcID := "vpc-id-1"

	mockvpc.EXPECT().
		GetVPCRoutingTable(gomock.Any()).
		Return(routingTableStable(rtID, rtName), &core.DetailedResponse{StatusCode: 200}, nil)

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		VPC: &infrav1.VPCResource{ID: &vpcID},
		RoutingTables: []infrav1.VPCRoutingTable{
			{ID: &rtID},
		},
	})
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	requeue, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).To(BeNil())
	g.Expect(requeue).To(BeFalse())
	// Verify status was updated
	g.Expect(scope.NetworkStatus()).NotTo(BeNil())
	g.Expect(scope.NetworkStatus().RoutingTables).To(HaveKey(rtName))
	g.Expect(scope.NetworkStatus().RoutingTables[rtName].ID).To(Equal(rtID))
	g.Expect(scope.NetworkStatus().RoutingTables[rtName].Ready).To(BeTrue())
}

func TestReconcileRoutingTables_FoundByID_Pending(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	rtID := "rt-pending1"
	rtName := "pending-table"
	vpcID := "vpc-id-2"

	mockvpc.EXPECT().
		GetVPCRoutingTable(gomock.Any()).
		Return(routingTablePending(rtID, rtName), &core.DetailedResponse{StatusCode: 200}, nil)

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		VPC: &infrav1.VPCResource{ID: &vpcID},
		RoutingTables: []infrav1.VPCRoutingTable{
			{ID: &rtID},
		},
	})
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	requeue, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).To(BeNil())
	g.Expect(requeue).To(BeTrue())
	g.Expect(scope.NetworkStatus().RoutingTables[rtName].Ready).To(BeFalse())
}

func TestReconcileRoutingTables_FoundByID_Error(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	rtID := "rt-err-id"
	vpcID := "vpc-id-3"

	mockvpc.EXPECT().
		GetVPCRoutingTable(gomock.Any()).
		Return(nil, &core.DetailedResponse{StatusCode: 500}, errors.New("API error"))

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		VPC: &infrav1.VPCResource{ID: &vpcID},
		RoutingTables: []infrav1.VPCRoutingTable{
			{ID: &rtID},
		},
	})
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	_, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).NotTo(BeNil())
	g.Expect(err.Error()).To(ContainSubstring("error retrieving routing table by id"))
}

func TestReconcileRoutingTables_FoundByName_Stable(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	rtID := "rt-name-found"
	rtName := "named-table"
	vpcID := "vpc-id-4"

	mockvpc.EXPECT().
		GetVPCRoutingTableByName(vpcID, rtName).
		Return(routingTableStable(rtID, rtName), nil)

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		VPC: &infrav1.VPCResource{ID: &vpcID},
		RoutingTables: []infrav1.VPCRoutingTable{
			{Name: &rtName},
		},
	})
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	requeue, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).To(BeNil())
	g.Expect(requeue).To(BeFalse())
	g.Expect(scope.NetworkStatus().RoutingTables[rtName].ID).To(Equal(rtID))
	g.Expect(scope.NetworkStatus().RoutingTables[rtName].Ready).To(BeTrue())
}

func TestReconcileRoutingTables_NotFound_Creates(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	rtName := "new-routing-table"
	vpcID := "vpc-id-5"
	createdID := "rt-created-id"

	// Name lookup returns nil (not found)
	mockvpc.EXPECT().
		GetVPCRoutingTableByName(vpcID, rtName).
		Return(nil, nil)

	// Create is then called — return without CRN to skip tagging in this unit test.
	mockvpc.EXPECT().
		CreateVPCRoutingTable(gomock.Any()).
		DoAndReturn(func(opts *vpcv1.CreateVPCRoutingTableOptions) (*vpcv1.RoutingTable, *core.DetailedResponse, error) {
			g.Expect(opts.VPCID).To(Equal(&vpcID))
			g.Expect(opts.Name).To(Equal(&rtName))
			rt := routingTablePending(createdID, rtName)
			rt.CRN = nil // no CRN avoids TagResource call
			return rt, &core.DetailedResponse{StatusCode: 201}, nil
		})

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		VPC: &infrav1.VPCResource{ID: &vpcID},
		RoutingTables: []infrav1.VPCRoutingTable{
			{Name: &rtName},
		},
	})
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	requeue, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).To(BeNil())
	// Requeue is expected because routing table was just created.
	g.Expect(requeue).To(BeTrue())
	g.Expect(scope.NetworkStatus().RoutingTables[rtName].ID).To(Equal(createdID))
	g.Expect(scope.NetworkStatus().RoutingTables[rtName].Ready).To(BeFalse())
}

func TestReconcileRoutingTables_NotFound_CreateError(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	rtName := "fail-table"
	vpcID := "vpc-id-6"

	mockvpc.EXPECT().
		GetVPCRoutingTableByName(vpcID, rtName).
		Return(nil, nil)

	mockvpc.EXPECT().
		CreateVPCRoutingTable(gomock.Any()).
		Return(nil, &core.DetailedResponse{StatusCode: 500}, errors.New("create failed"))

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		VPC: &infrav1.VPCResource{ID: &vpcID},
		RoutingTables: []infrav1.VPCRoutingTable{
			{Name: &rtName},
		},
	})
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	_, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).NotTo(BeNil())
	g.Expect(err.Error()).To(ContainSubstring("failed to create routing table"))
}

func TestReconcileRoutingTables_StatusPresent_LooksUpExisting(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	rtID := "rt-already-tracked"
	rtName := "tracked-table"
	vpcID := "vpc-id-7"

	// Status already tracks this routing table
	mockvpc.EXPECT().
		GetVPCRoutingTable(gomock.Any()).
		DoAndReturn(func(opts *vpcv1.GetVPCRoutingTableOptions) (*vpcv1.RoutingTable, *core.DetailedResponse, error) {
			g.Expect(*opts.ID).To(Equal(rtID))
			return routingTableStable(rtID, rtName), &core.DetailedResponse{StatusCode: 200}, nil
		})

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		VPC: &infrav1.VPCResource{ID: &vpcID},
		RoutingTables: []infrav1.VPCRoutingTable{
			{Name: &rtName},
		},
	})
	// Pre-populate network status as if a previous reconciliation ran.
	cluster.Status.Network = &infrav1.VPCNetworkStatus{
		VPC: &infrav1.ResourceStatus{ID: vpcID, Ready: true},
		RoutingTables: map[string]*infrav1.ResourceStatus{
			rtName: {ID: rtID, Name: ptr.To(rtName), Ready: false},
		},
	}
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	requeue, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).To(BeNil())
	g.Expect(requeue).To(BeFalse())
	g.Expect(scope.NetworkStatus().RoutingTables[rtName].Ready).To(BeTrue())
}

func TestBuildRoutingTableRoutes_DeliverAction(t *testing.T) {
	g := NewWithT(t)

	scope := &ClusterScopeV2{}
	nextHop := "10.0.0.1"
	routes := []infrav1.VPCRoutingTableRoute{
		{
			Action:      "deliver",
			Destination: "192.168.0.0/24",
			Zone:        "us-south-1",
			NextHop:     &nextHop,
			Name:        ptr.To("my-route"),
		},
	}

	sdkRoutes, err := scope.buildRoutingTableRoutes(routes)
	g.Expect(err).To(BeNil())
	g.Expect(sdkRoutes).To(HaveLen(1))
	g.Expect(*sdkRoutes[0].Action).To(Equal("deliver"))
	g.Expect(*sdkRoutes[0].Destination).To(Equal("192.168.0.0/24"))
	g.Expect(*sdkRoutes[0].Name).To(Equal("my-route"))
	g.Expect(sdkRoutes[0].NextHop).NotTo(BeNil())
}

func TestBuildRoutingTableRoutes_DropAction(t *testing.T) {
	g := NewWithT(t)

	scope := &ClusterScopeV2{}
	routes := []infrav1.VPCRoutingTableRoute{
		{
			Action:      "drop",
			Destination: "10.0.0.0/8",
			Zone:        "us-south-2",
		},
	}

	sdkRoutes, err := scope.buildRoutingTableRoutes(routes)
	g.Expect(err).To(BeNil())
	g.Expect(sdkRoutes).To(HaveLen(1))
	g.Expect(*sdkRoutes[0].Action).To(Equal("drop"))
	g.Expect(sdkRoutes[0].NextHop).To(BeNil())
}

func TestReconcileRoutingTables_VPCIDFromStatus(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	rtName := "table-from-status-vpc"
	vpcID := "vpc-from-status"
	rtID := "rt-from-status"

	// Expect name lookup after VPC ID is obtained from status.
	mockvpc.EXPECT().
		GetVPCRoutingTableByName(vpcID, rtName).
		Return(routingTableStable(rtID, rtName), nil)

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		// No VPC in spec — VPC ID comes from Status.
		RoutingTables: []infrav1.VPCRoutingTable{
			{Name: &rtName},
		},
	})
	// Set VPC status to simulate prior VPC reconciliation.
	cluster.Status.Network = &infrav1.VPCNetworkStatus{
		VPC: &infrav1.ResourceStatus{ID: vpcID, Ready: true},
	}
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	requeue, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).To(BeNil())
	g.Expect(requeue).To(BeFalse())
	g.Expect(scope.NetworkStatus().RoutingTables[rtName].ID).To(Equal(rtID))
}

func TestReconcileRoutingTables_AdvertiseRoutesTo_IsPassedToAPI(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	rtName := "rt-advertise-test"
	vpcID := "vpc-id-advertise"
	createdID := "rt-advertise-id"

	mockvpc.EXPECT().
		GetVPCRoutingTableByName(vpcID, rtName).
		Return(nil, nil)

	mockvpc.EXPECT().
		CreateVPCRoutingTable(gomock.Any()).
		DoAndReturn(func(opts *vpcv1.CreateVPCRoutingTableOptions) (*vpcv1.RoutingTable, *core.DetailedResponse, error) {
			g.Expect(opts.AdvertiseRoutesTo).To(ConsistOf("transit_gateway"))
			rt := routingTablePending(createdID, rtName)
			rt.CRN = nil
			return rt, &core.DetailedResponse{StatusCode: 201}, nil
		})

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		VPC: &infrav1.VPCResource{ID: &vpcID},
		RoutingTables: []infrav1.VPCRoutingTable{
			{
				Name:              &rtName,
				AdvertiseRoutesTo: []string{"transit_gateway"},
			},
		},
	})
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	requeue, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).To(BeNil())
	g.Expect(requeue).To(BeTrue())
}

func TestReconcileRoutingTables_NoAdvertiseRoutesTo_NotPassedToAPI(t *testing.T) {
	g := NewWithT(t)
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	mockvpc := mock.NewMockVpc(ctrl)

	rtName := "rt-no-advertise-test"
	vpcID := "vpc-id-no-advertise"
	createdID := "rt-no-advertise-id"

	mockvpc.EXPECT().
		GetVPCRoutingTableByName(vpcID, rtName).
		Return(nil, nil)

	mockvpc.EXPECT().
		CreateVPCRoutingTable(gomock.Any()).
		DoAndReturn(func(opts *vpcv1.CreateVPCRoutingTableOptions) (*vpcv1.RoutingTable, *core.DetailedResponse, error) {
			g.Expect(opts.AdvertiseRoutesTo).To(BeEmpty())
			rt := routingTablePending(createdID, rtName)
			rt.CRN = nil
			return rt, &core.DetailedResponse{StatusCode: 201}, nil
		})

	cluster := newIBMVPCClusterWithNetwork(clusterName, &infrav1.VPCNetworkSpec{
		VPC: &infrav1.VPCResource{ID: &vpcID},
		RoutingTables: []infrav1.VPCRoutingTable{
			{Name: &rtName},
		},
	})
	scope := setupClusterScopeV2WithMock(cluster, mockvpc)

	requeue, err := scope.ReconcileRoutingTables(context.Background())
	g.Expect(err).To(BeNil())
	g.Expect(requeue).To(BeTrue())
}
