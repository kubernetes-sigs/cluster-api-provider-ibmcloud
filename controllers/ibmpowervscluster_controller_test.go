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

package controllers

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"go.uber.org/mock/gomock"

	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	tgapiv1 "github.com/IBM/networking-go-sdk/transitgatewayapisv1"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	powervsmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	resourceclientmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"
	tgmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/transitgateway/mock"
	vpcmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func TestIBMPowerVSClusterReconciler_Reconcile(t *testing.T) {
	t.Run("Should fail Reconcile if owner cluster not found", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1beta2.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: capiv1beta1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "capi-test",
						UID:        "1",
					}}},
			Spec: infrav1beta2.IBMPowerVSClusterSpec{ServiceInstanceID: "foo"},
		}

		createCluster(g, powerVSCluster, ns.Name)
		defer cleanupCluster(g, powerVSCluster, ns)

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: powerVSCluster.Namespace,
				Name:      powerVSCluster.Name,
			},
		})

		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Should not reconcile if owner reference is not set", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1beta2.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-"},
			Spec: infrav1beta2.IBMPowerVSClusterSpec{ServiceInstanceID: "foo"},
		}

		createCluster(g, powerVSCluster, ns.Name)
		defer cleanupCluster(g, powerVSCluster, ns)

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: powerVSCluster.Namespace,
				Name:      powerVSCluster.Name,
			},
		})

		g.Expect(err).To(BeNil())
	})

	t.Run("Should Reconcile successfully if no IBMPowerVSCluster found", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: ns.Name,
				Name:      "test-cluster",
			},
		})

		g.Expect(err).To(BeNil())
	})

	t.Run("Error creating cluster scope", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1beta2.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: capiv1beta1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "capi-test",
						UID:        "1",
					}}},
			Spec: infrav1beta2.IBMPowerVSClusterSpec{ServiceInstanceID: "foo"},
		}

		createCluster(g, powerVSCluster, ns.Name)
		defer cleanupCluster(g, powerVSCluster, ns)

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: powerVSCluster.Namespace,
				Name:      powerVSCluster.Name,
			},
		})
		g.Expect(err).ToNot(BeNil())
	})

	t.Run("Successfully reconcile", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1beta2.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: capiv1beta1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "capi-test",
						UID:        "1",
					}}},
			Spec: infrav1beta2.IBMPowerVSClusterSpec{Zone: ptr.To("zone")},
		}

		ownerCluster := &capiv1beta1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "capi-test",
				Namespace: ns.Name,
			},
		}

		g.Expect(testEnv.Create(ctx, ownerCluster)).To(Succeed())
		defer func(obj ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, obj...)).To(Succeed())
		}(ownerCluster)

		createCluster(g, powerVSCluster, ns.Name)
		defer cleanupCluster(g, powerVSCluster, ns)

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
			ClientFactory: scope.ClientFactory{
				PowerVSClientFactory: func() (powervs.PowerVS, error) {
					return nil, nil
				},
				AuthenticatorFactory: func() (core.Authenticator, error) {
					return nil, nil
				},
			},
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: powerVSCluster.Namespace,
				Name:      powerVSCluster.Name,
			},
		})
		g.Expect(err).To(BeNil())

		ibmPowerVSCluster := &infrav1beta2.IBMPowerVSCluster{}
		g.Eventually(func(gomega Gomega) {
			gomega.Expect(testEnv.Client.Get(ctx, client.ObjectKey{
				Name:      powerVSCluster.GetName(),
				Namespace: powerVSCluster.GetNamespace(),
			}, ibmPowerVSCluster)).To(Succeed())
			gomega.Expect(len(ibmPowerVSCluster.Finalizers)).To(Equal(1))
		}).Should(Succeed())
	})

	t.Run("Successfully call reconcile delete", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1beta2.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				Finalizers:   []string{"ibmpowervscluster.infrastructure.cluster.x-k8s.io"},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: capiv1beta1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "capi-test",
						UID:        "1",
					}}},
			Spec: infrav1beta2.IBMPowerVSClusterSpec{Zone: ptr.To("zone")},
		}

		ownerCluster := &capiv1beta1.Cluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "capi-test",
				Namespace: ns.Name,
			},
		}

		g.Expect(testEnv.Create(ctx, ownerCluster)).To(Succeed())
		defer func(obj ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, obj...)).To(Succeed())
		}(ownerCluster)

		createCluster(g, powerVSCluster, ns.Name)
		defer cleanupCluster(g, powerVSCluster, ns)

		g.Expect(testEnv.Delete(ctx, powerVSCluster)).To(Succeed())
		ibmPowerVSCluster := &infrav1beta2.IBMPowerVSCluster{}

		g.Eventually(func() bool {
			err := testEnv.Client.Get(ctx, client.ObjectKey{
				Name:      powerVSCluster.GetName(),
				Namespace: powerVSCluster.GetNamespace(),
			}, ibmPowerVSCluster)
			g.Expect(err).To(BeNil())
			return ibmPowerVSCluster.DeletionTimestamp.IsZero() == false
		}, 10*time.Second).Should(BeTrue(), "Eventually failed while checking delete timestamp")

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
			ClientFactory: scope.ClientFactory{
				PowerVSClientFactory: func() (powervs.PowerVS, error) {
					return nil, nil
				},
				AuthenticatorFactory: func() (core.Authenticator, error) {
					return nil, nil
				},
			},
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: powerVSCluster.Namespace,
				Name:      powerVSCluster.Name,
			},
		})
		g.Expect(err).To(BeNil())
	})
}

func TestIBMPowerVSClusterReconciler_reconcile(t *testing.T) {
	testCases := []struct {
		name                string
		powervsClusterScope func() *scope.PowerVSClusterScope
		clusterStatus       bool
		expectedResult      ctrl.Result
		expectedError       error
		conditions          capiv1beta1.Conditions
	}{
		{
			name: "Should add finalizer and reconcile IBMPowerVSCluster",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				return &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
				}
			},
		},
		{
			name: "Should reconcile IBMPowerVSCluster status as Ready",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				return &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers: []string{infrav1beta2.IBMPowerVSClusterFinalizer},
						},
					},
				}
			},
			clusterStatus: true,
		},
		{
			name: "When PowerVS zone does not support PER",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers:  []string{infrav1beta2.IBMPowerVSClusterFinalizer},
							Annotations: map[string]string{infrav1beta2.CreateInfrastructureAnnotation: "true"},
						},
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							Zone: ptr.To("dal10"),
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatacenterCapabilities(gomock.Any()).Return(map[string]bool{"power-edge-router": false}, nil)
				clusterScope.IBMPowerVSClient = mockPowerVS
				return clusterScope
			},
			expectedError: errors.New("power-edge-router is not available for zone: dal10"),
		},
		{
			name: "When resource group name is not set",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers:  []string{infrav1beta2.IBMPowerVSClusterFinalizer},
							Annotations: map[string]string{infrav1beta2.CreateInfrastructureAnnotation: "true"},
						},
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							Zone: ptr.To("dal10"),
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatacenterCapabilities(gomock.Any()).Return(map[string]bool{"power-edge-router": true}, nil)
				clusterScope.IBMPowerVSClient = mockPowerVS
				return clusterScope
			},
			expectedError: errors.New("resource group name is not set"),
		},
		{
			name: "When reconcile PowerVS resource returns requeue as true",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers:  []string{infrav1beta2.IBMPowerVSClusterFinalizer},
							Annotations: map[string]string{infrav1beta2.CreateInfrastructureAnnotation: "true"},
						},
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							Zone: ptr.To("dal10"),
							ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("rg-id"),
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							ServiceInstance: &infrav1beta2.ResourceReference{
								ID: ptr.To("serviceInstanceID"),
							},
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatacenterCapabilities(gomock.Any()).Return(map[string]bool{"power-edge-router": true}, nil)
				clusterScope.IBMPowerVSClient = mockPowerVS

				mockResourceClient := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					Name:  ptr.To("serviceInstanceName"),
					ID:    ptr.To("serviceInstanceID"),
					State: ptr.To(string(infrav1beta2.ServiceInstanceStateProvisioning)),
				}, nil, nil)
				clusterScope.ResourceClient = mockResourceClient

				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, errors.New("vpc not found"))
				clusterScope.IBMVPCClient = mockVPC

				return clusterScope
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
		},
		{
			name: "When reconcile PowerVS and VPC resource returns requeue as true",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers:  []string{infrav1beta2.IBMPowerVSClusterFinalizer},
							Annotations: map[string]string{infrav1beta2.CreateInfrastructureAnnotation: "true"},
						},
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							Zone: ptr.To("dal10"),
							ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("rg-id"),
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							ServiceInstance: &infrav1beta2.ResourceReference{
								ID: ptr.To("serviceInstanceID"),
							},
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatacenterCapabilities(gomock.Any()).Return(map[string]bool{"power-edge-router": true}, nil)
				clusterScope.IBMPowerVSClient = mockPowerVS

				mockResourceClient := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					Name:  ptr.To("serviceInstanceName"),
					ID:    ptr.To("serviceInstanceID"),
					State: ptr.To(string(infrav1beta2.ServiceInstanceStateProvisioning)),
				}, nil, nil)
				clusterScope.ResourceClient = mockResourceClient

				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("pending")}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC

				return clusterScope
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
		},
		{
			name: "When reconcile VPC and PowerVS resource returns error",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers:  []string{infrav1beta2.IBMPowerVSClusterFinalizer},
							Annotations: map[string]string{infrav1beta2.CreateInfrastructureAnnotation: "true"},
						},
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							Zone: ptr.To("dal10"),
							ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("rg-id"),
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							ServiceInstance: &infrav1beta2.ResourceReference{
								ID: ptr.To("serviceInstanceID"),
							},
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatacenterCapabilities(gomock.Any()).Return(map[string]bool{"power-edge-router": true}, nil)
				clusterScope.IBMPowerVSClient = mockPowerVS

				mockResourceClient := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("error getting resource instance"))
				clusterScope.ResourceClient = mockResourceClient

				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, errors.New("vpc not found"))
				clusterScope.IBMVPCClient = mockVPC

				return clusterScope
			},
			expectedError: kerrors.NewAggregate([]error{errors.New("error getting resource instance"), errors.New("vpc not found")}),
		},
		{
			name: "When reconcile TransitGateway returns error",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: getPowerVSClusterWithSpecAndStatus(),
				}
				clusterScope.IBMPowerVSClient = getMockPowerVS(t)

				mockResourceClient := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					Name:  ptr.To("serviceInstanceName"),
					ID:    ptr.To("serviceInstanceID"),
					State: ptr.To(string(infrav1beta2.ServiceInstanceStateActive)),
				}, nil, nil)
				clusterScope.ResourceClient = mockResourceClient

				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To(string(infrav1beta2.VPCLoadBalancerStateActive))}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
					ID:                 ptr.To("lb-id"),
					Name:               ptr.To("lb"),
					ProvisioningStatus: ptr.To(string(infrav1beta2.VPCLoadBalancerStateActive)),
				}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC

				mockTransitGateway := tgmock.NewMockTransitGateway(gomock.NewController(t))
				mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(nil, nil, errors.New("error getting transitGateway"))
				clusterScope.TransitGatewayClient = mockTransitGateway

				return clusterScope
			},
			expectedError: errors.New("error getting transitGateway"),
			conditions: capiv1beta1.Conditions{
				getVPCLBReadyCondition(),
				getNetworkReadyCondition(),
				getServiceInstanceReadyCondition(),
				capiv1beta1.Condition{
					Type:               infrav1beta2.TransitGatewayReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.TransitGatewayReconciliationFailedReason,
					Message:            "error getting transitGateway",
				},
				getVPCReadyCondition(),
				getVPCSGReadyCondition(),
				getVPCSubnetReadyCondition(),
			},
		},
		{
			name: "When reconcile TransitGateway returns requeue as true",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: getPowerVSClusterWithSpecAndStatus(),
				}

				clusterScope.IBMPowerVSClient = getMockPowerVS(t)

				mockResourceClient := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					Name:  ptr.To("serviceInstanceName"),
					ID:    ptr.To("serviceInstanceID"),
					State: ptr.To(string(infrav1beta2.ServiceInstanceStateActive)),
				}, nil, nil)
				clusterScope.ResourceClient = mockResourceClient

				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
					ID:                 ptr.To("lb-id"),
					Name:               ptr.To("lb"),
					ProvisioningStatus: ptr.To(string(infrav1beta2.VPCLoadBalancerStateActive)),
				}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC

				mockTransitGateway := tgmock.NewMockTransitGateway(gomock.NewController(t))
				mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{
					Name:   ptr.To("transitGateway"),
					ID:     ptr.To("transitGatewayID"),
					Status: ptr.To(string(infrav1beta2.TransitGatewayStatePending)),
				}, nil, nil)
				clusterScope.TransitGatewayClient = mockTransitGateway

				return clusterScope
			},
			expectedResult: ctrl.Result{RequeueAfter: time.Minute},
		},
		{
			name: "When reconcile COS service instance returns error",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				powerVSCluster := getPowerVSClusterWithSpecAndStatus()
				powerVSCluster.Spec.Ignition = &infrav1beta2.Ignition{Version: "3.4"}
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: powerVSCluster,
				}
				clusterScope.IBMPowerVSClient = getMockPowerVS(t)
				mockResourceClient := getMockResourceController(t)
				mockResourceClient.EXPECT().GetInstanceByName(gomock.Any(), gomock.Any(), gomock.Any()).Return(nil, errors.New("error getting instance by name"))
				clusterScope.ResourceClient = mockResourceClient
				clusterScope.IBMVPCClient = getMockVPC(t)
				clusterScope.TransitGatewayClient = getMockTransitGateway(t)

				return clusterScope
			},
			expectedError: errors.New("error getting instance by name"),
			conditions: capiv1beta1.Conditions{
				capiv1beta1.Condition{
					Type:               infrav1beta2.COSInstanceReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.COSInstanceReconciliationFailedReason,
					Message:            "error getting instance by name",
				},
				getVPCLBReadyCondition(),
				getNetworkReadyCondition(),
				getServiceInstanceReadyCondition(),
				getTGReadyCondition(),
				getVPCReadyCondition(),
				getVPCSGReadyCondition(),
				getVPCSubnetReadyCondition(),
			},
		},
		{
			name: "When reconcile network is not ready",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: getPowerVSClusterWithSpecAndStatus(),
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatacenterCapabilities(gomock.Any()).Return(map[string]bool{"power-edge-router": true}, nil)
				mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(&models.DHCPServerDetail{
					ID:     ptr.To("dhcpID"),
					Status: ptr.To(string(infrav1beta2.DHCPServerStateBuild)),
				}, nil)
				mockPowerVS.EXPECT().WithClients(gomock.Any())
				clusterScope.IBMPowerVSClient = mockPowerVS

				clusterScope.ResourceClient = getMockResourceController(t)
				clusterScope.IBMVPCClient = getMockVPC(t)
				clusterScope.TransitGatewayClient = getMockTransitGateway(t)

				return clusterScope
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
		},
		{
			name: "When getting loadbalancer hostname returns error",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: getPowerVSClusterWithSpecAndStatus(),
				}
				clusterScope.IBMPowerVSClient = getMockPowerVS(t)
				clusterScope.ResourceClient = getMockResourceController(t)
				clusterScope.TransitGatewayClient = getMockTransitGateway(t)

				mockVPC := getMockVPC(t)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, nil, errors.New("failed to get loadbalancer"))
				clusterScope.IBMVPCClient = mockVPC

				return clusterScope
			},
			expectedError: fmt.Errorf("failed to fetch public loadbalancer: %w", errors.New("failed to get loadbalancer")),
		},
		{
			name: "When loadbalancer hostname is nil",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				powerVSCluster := getPowerVSClusterWithSpecAndStatus()
				powerVSCluster.Spec.LoadBalancers[0].Name = "lb-name"
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: powerVSCluster,
				}
				clusterScope.IBMPowerVSClient = getMockPowerVS(t)
				clusterScope.ResourceClient = getMockResourceController(t)
				clusterScope.IBMVPCClient = getMockVPC(t)
				clusterScope.TransitGatewayClient = getMockTransitGateway(t)

				return clusterScope
			},
			expectedResult: ctrl.Result{RequeueAfter: time.Minute},
		},
		{
			name: "When reconcile is successful",
			powervsClusterScope: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					Cluster:           &capiv1beta1.Cluster{},
					IBMPowerVSCluster: getPowerVSClusterWithSpecAndStatus(),
				}
				clusterScope.IBMPowerVSClient = getMockPowerVS(t)
				clusterScope.ResourceClient = getMockResourceController(t)
				clusterScope.TransitGatewayClient = getMockTransitGateway(t)

				mockVPC := getMockVPC(t)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
					ID:                 ptr.To("lb-id"),
					Name:               ptr.To("lb"),
					ProvisioningStatus: ptr.To(string(infrav1beta2.VPCLoadBalancerStateActive)),
					Hostname:           ptr.To("hostname"),
				}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC

				return clusterScope
			},
			clusterStatus: true,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMPowerVSClusterReconciler{
				Client: testEnv.Client,
			}
			powerVSClusterScope := tc.powervsClusterScope()
			res, err := reconciler.reconcile(powerVSClusterScope)
			if tc.expectedError != nil {
				g.Expect(err).To(Equal(tc.expectedError))
			} else {
				g.Expect(err).To(BeNil())
			}
			g.Expect(res).To(Equal(tc.expectedResult))
			g.Expect(powerVSClusterScope.IBMPowerVSCluster.Status.Ready).To(Equal(tc.clusterStatus))
			g.Expect(powerVSClusterScope.IBMPowerVSCluster.Finalizers).To(ContainElement(infrav1beta2.IBMPowerVSClusterFinalizer))
			if len(tc.conditions) > 1 {
				ignoreLastTransitionTime := cmp.Transformer("", func(metav1.Time) metav1.Time {
					return metav1.Time{}
				})
				g.Expect(powerVSClusterScope.IBMPowerVSCluster.GetConditions()).To(BeComparableTo(tc.conditions, ignoreLastTransitionTime))
			}
		})
	}
}

func TestIBMPowerVSClusterReconciler_delete(t *testing.T) {
	var (
		reconciler   IBMPowerVSClusterReconciler
		clusterScope *scope.PowerVSClusterScope
	)
	reconciler = IBMPowerVSClusterReconciler{
		Client: testEnv.Client,
	}
	t.Run("Reconciling delete IBMPowerVSCluster", func(t *testing.T) {
		t.Run("Should reconcile successfully if no descendants are found", func(t *testing.T) {
			g := NewWithT(t)
			clusterScope = &scope.PowerVSClusterScope{
				Logger: klog.Background(),
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					TypeMeta: metav1.TypeMeta{
						Kind:       "IBMPowerVSCluster",
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "capi-powervs-cluster",
					},
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						ServiceInstanceID: "service-instance-1",
					},
				},
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects().Build(),
			}
			result, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(result.RequeueAfter).To(BeZero())
		})
		t.Run("Should reconcile with requeue by deleting the cluster descendants", func(t *testing.T) {
			g := NewWithT(t)
			clusterScope = &scope.PowerVSClusterScope{
				Logger: klog.Background(),
				IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
					TypeMeta: metav1.TypeMeta{
						Kind:       "IBMPowerVSCluster",
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "capi-powervs-cluster",
					},
					Spec: infrav1beta2.IBMPowerVSClusterSpec{
						ServiceInstanceID: "service-instance-1",
					},
				},
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects().Build(),
			}
			powervsImage1 := &infrav1beta2.IBMPowerVSImage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-image",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: infrav1beta2.GroupVersion.String(),
							Kind:       "IBMPowerVSCluster",
							Name:       "capi-powervs-cluster",
							UID:        "1",
						},
					},
					Labels: map[string]string{capiv1beta1.ClusterNameLabel: "capi-powervs-cluster"},
				},
				Spec: infrav1beta2.IBMPowerVSImageSpec{
					ClusterName: "capi-powervs-cluster",
					Object:      ptr.To("capi-image.ova.gz"),
					Region:      ptr.To("us-south"),
					Bucket:      ptr.To("capi-bucket"),
				},
			}
			powervsImage2 := &infrav1beta2.IBMPowerVSImage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-image2",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: infrav1beta2.GroupVersion.String(),
							Kind:       "IBMPowerVSCluster",
							Name:       "capi-powervs-cluster",
							UID:        "1",
						},
					},
					Labels: map[string]string{capiv1beta1.ClusterNameLabel: "capi-powervs-cluster"},
				},
				Spec: infrav1beta2.IBMPowerVSImageSpec{
					ClusterName: "capi-powervs-cluster",
					Object:      ptr.To("capi-image2.ova.gz"),
					Region:      ptr.To("us-south"),
					Bucket:      ptr.To("capi-bucket"),
				},
			}
			createObject(g, powervsImage1, "default")
			defer cleanupObject(g, powervsImage1)
			createObject(g, powervsImage2, "default")
			defer cleanupObject(g, powervsImage2)

			result, err := reconciler.reconcileDelete(ctx, clusterScope)
			g.Expect(err).To(BeNil())
			g.Expect(result.RequeueAfter).To(Not(BeZero()))
			// Updating the object should fail as it doesn't exist
			g.Expect(clusterScope.Client.Update(ctx, powervsImage1)).To(Not(Succeed()))
			g.Expect(clusterScope.Client.Update(ctx, powervsImage2)).To(Not(Succeed()))
		})
	})
}

func TestReconcileVPCResources(t *testing.T) {
	testCases := []struct {
		name                    string
		powerVSClusterScopeFunc func() *scope.PowerVSClusterScope
		reconcileResult         reconcileResult
		conditions              capiv1beta1.Conditions
	}{
		{
			name: "when ReconcileVPC returns error",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{},
				}
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, errors.New("vpc not found"))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: errors.New("vpc not found"),
			},
			conditions: capiv1beta1.Conditions{
				capiv1beta1.Condition{
					Type:               infrav1beta2.VPCReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.VPCReconciliationFailedReason,
					Message:            "vpc not found",
				},
			},
		},
		{
			name: "when ReconcileVPC returns requeue as true",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("pending")}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				Result: reconcile.Result{
					Requeue: true,
				},
			},
		},
		{
			name: "when Reconciling VPC subnets returns error",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							VPC: &infrav1beta2.VPCResourceReference{
								Region: ptr.To("us-south"),
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, errors.New("vpc subnet not found"))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: errors.New("vpc subnet not found"),
			},

			conditions: capiv1beta1.Conditions{
				getVPCReadyCondition(),
				capiv1beta1.Condition{
					Type:               infrav1beta2.VPCSubnetReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.VPCSubnetReconciliationFailedReason,
					Message:            "vpc subnet not found",
				},
			},
		},
		{
			name: "when Reconciling VPC subnets returns requeue as true",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{
								ID: ptr.To("rg-id"),
							},
							VPC: &infrav1beta2.VPCResourceReference{
								Region: ptr.To("us-south"),
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil)
				mockVPC.EXPECT().GetSubnetAddrPrefix(gomock.Any(), gomock.Any()).Return("cidr", nil)
				mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(&vpcv1.Subnet{Status: ptr.To("active")}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				Result: reconcile.Result{
					Requeue: true,
				},
			},
			conditions: capiv1beta1.Conditions{
				getVPCReadyCondition(),
			},
		},
		{
			name: "when Reconciling VPC security group returns error",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							VPC: &infrav1beta2.VPCResourceReference{
								Region: ptr.To("us-south"),
							},
							VPCSubnets: []infrav1beta2.Subnet{
								{
									ID: ptr.To("subnet-id"),
								},
							},
							VPCSecurityGroups: []infrav1beta2.VPCSecurityGroup{
								{
									Name: ptr.To("security-group"),
								},
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, errors.New("vpc security group not found"))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: fmt.Errorf("failed to validate existing security group: vpc security group not found"),
			},

			conditions: capiv1beta1.Conditions{
				getVPCReadyCondition(),
				capiv1beta1.Condition{
					Type:               infrav1beta2.VPCSecurityGroupReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.VPCSecurityGroupReconciliationFailedReason,
					Message:            "failed to validate existing security group: vpc security group not found",
				},
				getVPCSubnetReadyCondition(),
			},
		},
		{
			name: "when Reconciling LoadBalancer returns error",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							VPC: &infrav1beta2.VPCResourceReference{
								Region: ptr.To("us-south"),
							},
							VPCSubnets: []infrav1beta2.Subnet{
								{
									ID: ptr.To("subnet-id"),
								},
							},
							LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
								{
									ID: ptr.To("lb-id"),
								},
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, nil, errors.New("loadbalancer not found"))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: fmt.Errorf("loadbalancer not found"),
			},

			conditions: capiv1beta1.Conditions{
				capiv1beta1.Condition{
					Type:               infrav1beta2.LoadBalancerReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.LoadBalancerReconciliationFailedReason,
					Message:            "loadbalancer not found",
				},
				getVPCReadyCondition(),
				getVPCSGReadyCondition(),
				getVPCSubnetReadyCondition(),
			},
		},
		{
			name: "when Reconciling LoadBalancer returns with loadbalancer status as ready",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							VPC: &infrav1beta2.VPCResourceReference{
								Region: ptr.To("us-south"),
							},
							VPCSubnets: []infrav1beta2.Subnet{
								{
									ID: ptr.To("subnet-id"),
								},
							},
							LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
								{
									ID: ptr.To("lb-id"),
								},
							},
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							VPC: &infrav1beta2.ResourceReference{
								ID: ptr.To("vpcID"),
							},
						},
					},
				}
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
					ID:                 ptr.To("lb-id"),
					Name:               ptr.To("lb"),
					ProvisioningStatus: ptr.To("active"),
				}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			conditions: capiv1beta1.Conditions{
				getVPCLBReadyCondition(),
				getVPCReadyCondition(),
				getVPCSGReadyCondition(),
				getVPCSubnetReadyCondition(),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMPowerVSClusterReconciler{
				Client: testEnv.Client,
			}
			clusterScope := tc.powerVSClusterScopeFunc()
			ch := make(chan reconcileResult, 1)
			pvsCluster := &powerVSCluster{
				cluster: clusterScope.IBMPowerVSCluster,
			}
			wg := &sync.WaitGroup{}
			wg.Add(1)
			reconciler.reconcileVPCResources(clusterScope, pvsCluster, ch, wg)
			wg.Wait()
			close(ch)
			g.Expect(<-ch).To(Equal(tc.reconcileResult))
			ignoreLastTransitionTime := cmp.Transformer("", func(metav1.Time) metav1.Time {
				return metav1.Time{}
			})
			g.Expect(pvsCluster.cluster.GetConditions()).To(BeComparableTo(tc.conditions, ignoreLastTransitionTime))
		})
	}
}

func TestReconcilePowerVSResources(t *testing.T) {
	testCases := []struct {
		name                    string
		powerVSClusterScopeFunc func() *scope.PowerVSClusterScope
		reconcileResult         reconcileResult
		conditions              capiv1beta1.Conditions
	}{
		{
			name: "When Reconciling PowerVS service instance returns error",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							ServiceInstance: &infrav1beta2.ResourceReference{
								ID: ptr.To("serviceInstanceID"),
							},
						},
					},
				}
				mockResourceController := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, fmt.Errorf("error getting resource instance"))
				clusterScope.ResourceClient = mockResourceController
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: errors.New("error getting resource instance"),
			},

			conditions: capiv1beta1.Conditions{
				capiv1beta1.Condition{
					Type:               infrav1beta2.ServiceInstanceReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.ServiceInstanceReconciliationFailedReason,
					Message:            "error getting resource instance",
				},
			},
		},
		{
			name: "When Reconciling PowerVS service instance returns requeue as true",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							ServiceInstance: &infrav1beta2.ResourceReference{
								ID: ptr.To("serviceInstanceID"),
							},
						},
					},
				}
				mockResourceController := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{State: ptr.To(string(infrav1beta2.ServiceInstanceStateProvisioning)), Name: ptr.To("serviceInstanceName")}, nil, nil)
				clusterScope.ResourceClient = mockResourceController
				return clusterScope
			},
			reconcileResult: reconcileResult{
				Result: reconcile.Result{
					Requeue: true,
				},
			},
		},
		{
			name: "When Reconciling network returns error",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							ServiceInstanceID: "serviceInstanceID",
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							DHCPServer:      &infrav1beta2.ResourceReference{ID: ptr.To("DHCPServerID")},
							ServiceInstance: &infrav1beta2.ResourceReference{ID: ptr.To("serviceInstanceID")},
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpID"), Status: ptr.To(string(infrav1beta2.DHCPServerStateError))}
				mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)
				mockPowerVS.EXPECT().WithClients(gomock.Any())
				mockResourceController := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{State: ptr.To(string(infrav1beta2.ServiceInstanceStateActive)), Name: ptr.To("serviceInstanceName")}, nil, nil)
				clusterScope.ResourceClient = mockResourceController
				clusterScope.IBMPowerVSClient = mockPowerVS
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: errors.New("DHCP server creation failed and is in error state"),
			},
			conditions: capiv1beta1.Conditions{
				capiv1beta1.Condition{
					Type:               infrav1beta2.NetworkReadyCondition,
					Status:             "False",
					Severity:           capiv1beta1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1beta2.NetworkReconciliationFailedReason,
					Message:            "DHCP server creation failed and is in error state",
				},
				getServiceInstanceReadyCondition(),
			},
		},
		{
			name: "When reconcile network returns with DHCP server in active state",
			powerVSClusterScopeFunc: func() *scope.PowerVSClusterScope {
				clusterScope := &scope.PowerVSClusterScope{
					IBMPowerVSCluster: &infrav1beta2.IBMPowerVSCluster{
						Spec: infrav1beta2.IBMPowerVSClusterSpec{
							ServiceInstanceID: "serviceInstanceID",
						},
						Status: infrav1beta2.IBMPowerVSClusterStatus{
							DHCPServer:      &infrav1beta2.ResourceReference{ID: ptr.To("DHCPServerID")},
							ServiceInstance: &infrav1beta2.ResourceReference{ID: ptr.To("serviceInstanceID")},
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				dhcpServer := &models.DHCPServerDetail{ID: ptr.To("dhcpID"), Status: ptr.To(string(infrav1beta2.DHCPServerStateActive))}
				mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(dhcpServer, nil)
				mockPowerVS.EXPECT().WithClients(gomock.Any())
				mockResourceController := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{State: ptr.To(string(infrav1beta2.ServiceInstanceStateActive)), Name: ptr.To("serviceInstanceName")}, nil, nil)
				clusterScope.ResourceClient = mockResourceController
				clusterScope.IBMPowerVSClient = mockPowerVS
				return clusterScope
			},
			conditions: capiv1beta1.Conditions{
				getNetworkReadyCondition(),
				getServiceInstanceReadyCondition(),
			},
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewWithT(t)
			reconciler := &IBMPowerVSClusterReconciler{
				Client: testEnv.Client,
			}
			clusterScope := tc.powerVSClusterScopeFunc()
			ch := make(chan reconcileResult, 1)
			pvsCluster := &powerVSCluster{
				cluster: clusterScope.IBMPowerVSCluster,
			}
			wg := &sync.WaitGroup{}
			wg.Add(1)
			reconciler.reconcilePowerVSResources(clusterScope, pvsCluster, ch, wg)
			wg.Wait()
			close(ch)
			g.Expect(<-ch).To(Equal(tc.reconcileResult))
			ignoreLastTransitionTime := cmp.Transformer("", func(metav1.Time) metav1.Time {
				return metav1.Time{}
			})
			g.Expect(pvsCluster.cluster.GetConditions()).To(BeComparableTo(tc.conditions, ignoreLastTransitionTime))
		})
	}
}

func getVPCReadyCondition() capiv1beta1.Condition {
	return capiv1beta1.Condition{
		Type:   infrav1beta2.VPCReadyCondition,
		Status: "True",
	}
}

func getVPCSubnetReadyCondition() capiv1beta1.Condition {
	return capiv1beta1.Condition{
		Type:   infrav1beta2.VPCSubnetReadyCondition,
		Status: "True",
	}
}

func getVPCSGReadyCondition() capiv1beta1.Condition {
	return capiv1beta1.Condition{
		Type:   infrav1beta2.VPCSecurityGroupReadyCondition,
		Status: "True",
	}
}

func getVPCLBReadyCondition() capiv1beta1.Condition {
	return capiv1beta1.Condition{
		Type:   infrav1beta2.LoadBalancerReadyCondition,
		Status: "True",
	}
}

func getTGReadyCondition() capiv1beta1.Condition {
	return capiv1beta1.Condition{
		Type:   infrav1beta2.TransitGatewayReadyCondition,
		Status: "True",
	}
}

func getPowerVSClusterWithSpecAndStatus() *infrav1beta2.IBMPowerVSCluster {
	return &infrav1beta2.IBMPowerVSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Finalizers:  []string{infrav1beta2.IBMPowerVSClusterFinalizer},
			Annotations: map[string]string{infrav1beta2.CreateInfrastructureAnnotation: "true"},
		},
		Spec: infrav1beta2.IBMPowerVSClusterSpec{
			Zone: ptr.To("dal10"),
			ResourceGroup: &infrav1beta2.IBMPowerVSResourceReference{
				ID: ptr.To("rg-id"),
			},
			VPC: &infrav1beta2.VPCResourceReference{
				Region: ptr.To("us-south"),
			},
			VPCSubnets: []infrav1beta2.Subnet{
				{
					ID: ptr.To("subnet-id"),
				},
			},
			LoadBalancers: []infrav1beta2.VPCLoadBalancerSpec{
				{
					ID:     ptr.To("lb-id"),
					Public: ptr.To(true),
				},
			},
		},
		Status: infrav1beta2.IBMPowerVSClusterStatus{
			ServiceInstance: &infrav1beta2.ResourceReference{
				ID: ptr.To("serviceInstanceID"),
			},
			DHCPServer: &infrav1beta2.ResourceReference{
				ID: ptr.To("DHCPServerID"),
			},
			VPC: &infrav1beta2.ResourceReference{
				ID: ptr.To("vpcID"),
			},
			TransitGateway: &infrav1beta2.TransitGatewayStatus{
				ID: ptr.To("transitGatewayID"),
			},
		},
	}
}

func getMockPowerVS(t *testing.T) *powervsmock.MockPowerVS {
	t.Helper()
	mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
	mockPowerVS.EXPECT().GetDatacenterCapabilities(gomock.Any()).Return(map[string]bool{"power-edge-router": true}, nil)
	mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(&models.DHCPServerDetail{
		ID:     ptr.To("dhcpID"),
		Status: ptr.To(string(infrav1beta2.DHCPServerStateActive)),
	}, nil)
	mockPowerVS.EXPECT().WithClients(gomock.Any())
	return mockPowerVS
}

func getMockResourceController(t *testing.T) *resourceclientmock.MockResourceController {
	t.Helper()
	mockResourceClient := resourceclientmock.NewMockResourceController(gomock.NewController(t))
	mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
		Name:  ptr.To("serviceInstanceName"),
		ID:    ptr.To("serviceInstanceID"),
		State: ptr.To("active"),
		CRN:   ptr.To("powervs_crn"),
	}, nil, nil).Times(2)
	return mockResourceClient
}

func getMockVPC(t *testing.T) *vpcmock.MockVpc {
	t.Helper()
	mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
	mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{
		Status: ptr.To("active"),
		CRN:    ptr.To("vpc_crn"),
	}, nil, nil).Times(2)
	mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
	mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
		ID:                 ptr.To("lb-id"),
		Name:               ptr.To("lb"),
		ProvisioningStatus: ptr.To("active"),
		Hostname:           ptr.To("hostname"),
	}, nil, nil)
	return mockVPC
}

func getMockTransitGateway(t *testing.T) *tgmock.MockTransitGateway {
	t.Helper()
	mockTransitGateway := tgmock.NewMockTransitGateway(gomock.NewController(t))
	mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{
		Name:   ptr.To("transitGateway"),
		ID:     ptr.To("transitGatewayID"),
		Status: ptr.To(string(infrav1beta2.TransitGatewayStateAvailable)),
	}, nil, nil)
	mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{
		Connections: []tgapiv1.TransitGatewayConnectionCust{
			{
				Name:        ptr.To("vpc_connection"),
				NetworkID:   ptr.To("vpc_crn"),
				NetworkType: ptr.To("vpc"),
				Status:      ptr.To(string(infrav1beta2.TransitGatewayConnectionStateAttached)),
			},
			{
				Name:        ptr.To("powervs_connection"),
				NetworkID:   ptr.To("powervs_crn"),
				NetworkType: ptr.To("power_virtual_server"),
				Status:      ptr.To(string(infrav1beta2.TransitGatewayConnectionStateAttached)),
			},
		},
	}, nil, nil)
	return mockTransitGateway
}

func createCluster(g *WithT, powervsCluster *infrav1beta2.IBMPowerVSCluster, namespace string) {
	if powervsCluster != nil {
		powervsCluster.Namespace = namespace
		g.Expect(testEnv.Create(ctx, powervsCluster)).To(Succeed())
		g.Eventually(func() bool {
			cluster := &infrav1beta2.IBMPowerVSCluster{}
			key := client.ObjectKey{
				Name:      powervsCluster.Name,
				Namespace: namespace,
			}
			err := testEnv.Get(ctx, key, cluster)
			return err == nil
		}, 10*time.Second).Should(Equal(true))
	}
}

func cleanupCluster(g *WithT, powervsCluster *infrav1beta2.IBMPowerVSCluster, namespace *corev1.Namespace) {
	if powervsCluster != nil {
		func(do ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
		}(powervsCluster, namespace)
	}
}

func getServiceInstanceReadyCondition() capiv1beta1.Condition {
	return capiv1beta1.Condition{
		Type:   infrav1beta2.ServiceInstanceReadyCondition,
		Status: "True",
	}
}
func getNetworkReadyCondition() capiv1beta1.Condition {
	return capiv1beta1.Condition{
		Type:   infrav1beta2.NetworkReadyCondition,
		Status: "True",
	}
}
