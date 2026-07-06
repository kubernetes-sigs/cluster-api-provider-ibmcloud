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
	"github.com/IBM/platform-services-go-sdk/resourcemanagerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	regionUtil "github.com/ppc64le-cloud/powervs-utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	powervsscope "sigs.k8s.io/cluster-api-provider-ibmcloud/cloud/scope/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	powervsmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs/mock"
	resourceclientmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller/mock"
	resourcemanagermock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcemanager/mock"
	tgmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/transitgateway/mock"
	vpcmock "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc/mock"

	. "github.com/onsi/gomega"
)

func TestIBMPowerVSClusterReconciler_Reconcile(t *testing.T) {
	t.Run("Should add the finalizer to IBMPowerVSCluster", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
			},
			Spec: infrav1.IBMPowerVSClusterSpec{
				Topology: infrav1.PowerVSVirtualIPTopology,
				Workspace: infrav1.WorkspaceSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "foo",
					},
				},
				Network: infrav1.NetworkSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "network-id",
					},
				},
			},
		}

		createCluster(g, powerVSCluster, ns.Name)
		defer cleanupCluster(g, powerVSCluster, ns)

		reconciler := &IBMPowerVSClusterReconciler{
			Client: testEnv.Client,
		}
		_, err = reconciler.Reconcile(ctx, ctrl.Request{
			NamespacedName: client.ObjectKey{
				Namespace: powerVSCluster.GetNamespace(),
				Name:      powerVSCluster.GetName(),
			},
		})
		g.Expect(err).To(BeNil())
		g.Eventually(func(gomega Gomega) {
			gomega.Expect(testEnv.Client.Get(ctx, client.ObjectKey{
				Namespace: powerVSCluster.GetNamespace(),
				Name:      powerVSCluster.GetName(),
			}, powerVSCluster)).To(Succeed())
			gomega.Expect(len(powerVSCluster.Finalizers)).To(Equal(1))
		}).Should(Succeed())
	})

	t.Run("Should fail Reconcile if owner cluster not found", func(t *testing.T) {
		g := NewWithT(t)

		ns, err := testEnv.CreateNamespace(ctx, fmt.Sprintf("namespace-%s", util.RandomString(5)))
		g.Expect(err).To(BeNil())

		powerVSCluster := &infrav1.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				Finalizers:   []string{infrav1.IBMPowerVSClusterFinalizer},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "capi-test",
						UID:        "1",
					}}},
			Spec: infrav1.IBMPowerVSClusterSpec{
				Topology: infrav1.PowerVSVirtualIPTopology,
				Workspace: infrav1.WorkspaceSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "foo",
					},
				},
				Network: infrav1.NetworkSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "network-id",
					},
				},
			},
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

		powerVSCluster := &infrav1.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				Finalizers:   []string{infrav1.IBMPowerVSClusterFinalizer},
			},
			Spec: infrav1.IBMPowerVSClusterSpec{
				Topology: infrav1.PowerVSVirtualIPTopology,
				Workspace: infrav1.WorkspaceSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "foo",
					},
				},
				Network: infrav1.NetworkSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "network-id",
					},
				},
			},
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

		powerVSCluster := &infrav1.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				Finalizers:   []string{infrav1.IBMPowerVSClusterFinalizer},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "capi-test",
						UID:        "1",
					}}},
			Spec: infrav1.IBMPowerVSClusterSpec{
				Topology: infrav1.PowerVSVirtualIPTopology,
				Workspace: infrav1.WorkspaceSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "foo",
					},
				},
				Network: infrav1.NetworkSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "network-id",
					},
				},
			},
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

		powerVSCluster := &infrav1.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				Finalizers:   []string{infrav1.IBMPowerVSClusterFinalizer},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "capi-test",
						UID:        "1",
					}}},
			Spec: infrav1.IBMPowerVSClusterSpec{
				Topology: infrav1.PowerVSVirtualIPTopology,
				Zone:     "zone",
				Workspace: infrav1.WorkspaceSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "workspace-id",
					},
				},
				Network: infrav1.NetworkSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "network-id",
					},
				},
			},
		}

		ownerCluster := &clusterv1.Cluster{
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
			ClientFactory: powervsscope.ClientFactory{
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

		ibmPowerVSCluster := &infrav1.IBMPowerVSCluster{}
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

		powerVSCluster := &infrav1.IBMPowerVSCluster{
			ObjectMeta: metav1.ObjectMeta{
				GenerateName: "powervs-test-",
				Finalizers:   []string{infrav1.IBMPowerVSClusterFinalizer},
				OwnerReferences: []metav1.OwnerReference{
					{
						APIVersion: clusterv1.GroupVersion.String(),
						Kind:       "Cluster",
						Name:       "capi-test",
						UID:        "1",
					}}},
			Spec: infrav1.IBMPowerVSClusterSpec{
				Topology: infrav1.PowerVSVirtualIPTopology,
				Zone:     "zone",
				Workspace: infrav1.WorkspaceSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "workspace-id",
					},
				},
				Network: infrav1.NetworkSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "network-id",
					},
				},
			},
		}

		ownerCluster := &clusterv1.Cluster{
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
		ibmPowerVSCluster := &infrav1.IBMPowerVSCluster{}

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
			ClientFactory: powervsscope.ClientFactory{
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
		powervsClusterScope func() *powervsscope.ClusterScope
		clusterStatus       bool
		expectedResult      ctrl.Result
		expectedError       error
		conditions          clusterv1.Conditions
	}{
		{
			name: "Should add finalizer and reconcile IBMPowerVSCluster",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				return &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers: []string{infrav1.IBMPowerVSClusterFinalizer},
						},
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSVirtualIPTopology,
						},
					},
				}
			},
			clusterStatus: true,
		},
		{
			name: "Should reconcile IBMPowerVSCluster status as Ready",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				return &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers: []string{infrav1.IBMPowerVSClusterFinalizer},
						},
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSVirtualIPTopology,
						},
					},
				}
			},
			clusterStatus: true,
		},
		{
			name: "When PowerVS zone does not support PER",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers:  []string{infrav1.IBMPowerVSClusterFinalizer},
							Annotations: map[string]string{infrav1.CreateInfrastructureAnnotation: "true"},
						},
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSLoadBalancerTopology,
							Zone:     "dal10",
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatatcenterDetails(gomock.Any()).Return(&models.Datacenter{
					Capabilities: map[string]bool{"power-edge-router": false},
				}, nil)
				clusterScope.IBMPowerVSClient = mockPowerVS
				return clusterScope
			},
			expectedError: errors.New("power-edge-router is not available for zone: dal10"),
		},
		{
			name: "When resource group name is not set",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers:  []string{infrav1.IBMPowerVSClusterFinalizer},
							Annotations: map[string]string{infrav1.CreateInfrastructureAnnotation: "true"},
						},
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSLoadBalancerTopology,
							Zone:     "dal10",
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatatcenterDetails(gomock.Any()).Return(&models.Datacenter{
					Capabilities: map[string]bool{"power-edge-router": true},
				}, nil)
				clusterScope.IBMPowerVSClient = mockPowerVS
				return clusterScope
			},
			expectedError: errors.New("resource group name is not set"),
		},
		{
			name: "When reconcile PowerVS resource returns requeue as true",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers:  []string{infrav1.IBMPowerVSClusterFinalizer},
							Annotations: map[string]string{infrav1.CreateInfrastructureAnnotation: "true"},
						},
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology:      infrav1.PowerVSLoadBalancerTopology,
							Zone:          "dal10",
							ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "rg-id"}},
							VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{Name: "vpc-name"}},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							Workspace: infrav1.ResourceReferenceV1Beta3{
								ID: "serviceInstanceID",
							},
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatatcenterDetails(gomock.Any()).Return(&models.Datacenter{
					Capabilities: map[string]bool{"power-edge-router": true},
				}, nil)
				clusterScope.IBMPowerVSClient = mockPowerVS

				mockResourceClient := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					Name:  ptr.To("serviceInstanceName"),
					ID:    ptr.To("serviceInstanceID"),
					State: ptr.To(string(infrav1.WorkspaceStateProvisioning)),
				}, nil, nil)
				clusterScope.ResourceClient = mockResourceClient

				clusterScope.ResourceManagerClient = getMockResourceManager(t)

				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, errors.New("vpc not found"))
				clusterScope.IBMVPCClient = mockVPC

				return clusterScope
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
		},
		{
			name: "When reconcile PowerVS and VPC resource returns requeue as true",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers:  []string{infrav1.IBMPowerVSClusterFinalizer},
							Annotations: map[string]string{infrav1.CreateInfrastructureAnnotation: "true"},
						},
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology:      infrav1.PowerVSLoadBalancerTopology,
							Zone:          "dal10",
							ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "rg-id"}},
							VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeProvision, Region: "us-south"},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							Workspace: infrav1.ResourceReferenceV1Beta3{
								ID: "serviceInstanceID",
							},
							VPC: infrav1.VPCStatus{
								ID:   "vpcID",
								Name: "vpcName",
							},
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatatcenterDetails(gomock.Any()).Return(&models.Datacenter{
					Capabilities: map[string]bool{"power-edge-router": true},
				}, nil)
				clusterScope.IBMPowerVSClient = mockPowerVS

				mockResourceClient := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					Name:  ptr.To("serviceInstanceName"),
					ID:    ptr.To("serviceInstanceID"),
					State: ptr.To(string(infrav1.WorkspaceStateProvisioning)),
				}, nil, nil)
				clusterScope.ResourceClient = mockResourceClient

				clusterScope.ResourceManagerClient = getMockResourceManager(t)

				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("pending")}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC

				return clusterScope
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
		},
		{
			name: "When reconcile VPC and PowerVS resource returns error",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						ObjectMeta: metav1.ObjectMeta{
							Finalizers:  []string{infrav1.IBMPowerVSClusterFinalizer},
							Annotations: map[string]string{infrav1.CreateInfrastructureAnnotation: "true"},
						},
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology:      infrav1.PowerVSLoadBalancerTopology,
							Zone:          "dal10",
							ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "rg-id"}},
							VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{Name: "vpc-name"}, Region: "us-south"},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							Workspace: infrav1.ResourceReferenceV1Beta3{
								ID: "serviceInstanceID",
							},
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatatcenterDetails(gomock.Any()).Return(&models.Datacenter{
					Capabilities: map[string]bool{"power-edge-router": true},
				}, nil)
				clusterScope.IBMPowerVSClient = mockPowerVS

				mockResourceClient := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("error getting resource instance"))
				clusterScope.ResourceClient = mockResourceClient

				clusterScope.ResourceManagerClient = getMockResourceManager(t)

				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, errors.New("vpc not found"))
				clusterScope.IBMVPCClient = mockVPC

				return clusterScope
			},
			expectedError: kerrors.NewAggregate([]error{
				fmt.Errorf("failed to reconcile VPC: %w", fmt.Errorf("failed to get referenced VPC: %w", errors.New("vpc not found"))),
				fmt.Errorf("failed to reconcile PowerVS workspace: %w", fmt.Errorf("failed to fetch workspace (id: serviceInstanceID) details: %w", errors.New("error getting resource instance"))),
			}),
		},
		{
			name: "When reconcile TransitGateway returns error",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: getPowerVSClusterWithSpecAndStatus(),
				}
				clusterScope.IBMPowerVSClient = getMockPowerVS(t)

				mockResourceClient := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					Name:  ptr.To("serviceInstanceName"),
					ID:    ptr.To("serviceInstanceID"),
					State: ptr.To(string(infrav1.WorkspaceStateActive)),
				}, nil, nil)
				clusterScope.ResourceClient = mockResourceClient

				clusterScope.ResourceManagerClient = getMockResourceManager(t)

				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To(string(infrav1.LoadBalancerStateActive))}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
					ID:                 ptr.To("lb-id"),
					Name:               ptr.To("lb"),
					ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
				}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC

				mockTransitGateway := tgmock.NewMockTransitGateway(gomock.NewController(t))
				mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(nil, nil, errors.New("error getting transit gateway"))
				clusterScope.TransitGatewayClient = mockTransitGateway

				return clusterScope
			},
			expectedError: fmt.Errorf("failed to reconcile transit gateway: %w", fmt.Errorf("failed to fetch transit gateway (id: transitGatewayID) details: %w", errors.New("error getting transit gateway"))),
			conditions: clusterv1.Conditions{
				getVPCLBReadyCondition(),
				getWorkspaceReadyCondition(),
				clusterv1.Condition{
					Type:               infrav1.TransitGatewayReadyCondition,
					Status:             "False",
					Severity:           clusterv1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1.TransitGatewayReconciliationFailedReason,
					Message:            "failed to fetch transit gateway (id: transitGatewayID) details: error getting transit gateway",
				},
				getVPCReadyCondition(),
				getVPCSGReadyCondition(),
				getVPCSubnetReadyCondition(),
			},
		},
		{
			name: "When reconcile TransitGateway returns requeue as true",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: getPowerVSClusterWithSpecAndStatus(),
				}

				clusterScope.IBMPowerVSClient = getMockPowerVS(t)

				mockResourceClient := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
					Name:  ptr.To("serviceInstanceName"),
					ID:    ptr.To("serviceInstanceID"),
					State: ptr.To(string(infrav1.WorkspaceStateActive)),
				}, nil, nil)
				clusterScope.ResourceClient = mockResourceClient

				clusterScope.ResourceManagerClient = getMockResourceManager(t)

				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
					ID:                 ptr.To("lb-id"),
					Name:               ptr.To("lb"),
					ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
				}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC

				mockTransitGateway := tgmock.NewMockTransitGateway(gomock.NewController(t))
				mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(&tgapiv1.TransitGateway{
					Name:   ptr.To("transitGateway"),
					ID:     ptr.To("transitGatewayID"),
					Status: ptr.To(string(infrav1.TransitGatewayStatePending)),
				}, nil, nil)
				clusterScope.TransitGatewayClient = mockTransitGateway

				return clusterScope
			},
			expectedResult: ctrl.Result{RequeueAfter: time.Minute},
		},
		{
			name: "When reconcile COS service instance returns error",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				powerVSCluster := getPowerVSClusterWithSpecAndStatus()
				powerVSCluster.Spec.Ignition = &infrav1.Ignition{Version: "3.4"}
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: powerVSCluster,
				}
				clusterScope.IBMPowerVSClient = getMockPowerVS(t)
				mockResourceClient := getMockResourceController(t)
				mockResourceClient.EXPECT().GetResourceInstanceByFilter(gomock.Any()).Return(nil, errors.New("error getting instance by name"))
				clusterScope.ResourceClient = mockResourceClient
				clusterScope.ResourceManagerClient = getMockResourceManager(t)
				clusterScope.IBMVPCClient = getMockVPC(t)
				clusterScope.TransitGatewayClient = getMockTransitGateway(t)

				return clusterScope
			},
			expectedError: errors.New("error getting instance by name"),
			conditions: clusterv1.Conditions{
				clusterv1.Condition{
					Type:               infrav1.COSInstanceReadyCondition,
					Status:             "False",
					Severity:           clusterv1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1.COSInstanceReconciliationFailedReason,
					Message:            "failed to check if COS instance in cloud: failed to get COS service instance: error getting instance by name",
				},
				getVPCLBReadyCondition(),
				getWorkspaceReadyCondition(),
				getTGReadyCondition(),
				getVPCReadyCondition(),
				getVPCSGReadyCondition(),
				getVPCSubnetReadyCondition(),
			},
		},
		{
			name: "When reconcile network is not ready",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: getPowerVSClusterWithSpecAndStatus(),
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetDatatcenterDetails(gomock.Any()).Return(&models.Datacenter{
					Capabilities: map[string]bool{"power-edge-router": true},
				}, nil)
				mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(nil, errors.New("error get networkByID"))
				mockPowerVS.EXPECT().WithClients(gomock.Any())
				clusterScope.IBMPowerVSClient = mockPowerVS

				clusterScope.ResourceClient = getMockResourceController(t)
				clusterScope.ResourceManagerClient = getMockResourceManager(t)
				clusterScope.IBMVPCClient = getMockVPC(t)
				clusterScope.TransitGatewayClient = getMockTransitGateway(t)

				return clusterScope
			},
			expectedResult: ctrl.Result{RequeueAfter: 30 * time.Second},
		},
		{
			name: "When getting loadbalancer hostname returns error",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				powerVSCluster := getPowerVSClusterWithSpecAndStatus()
				// Change to reference-type load balancer with only ID to trigger API call
				powerVSCluster.Spec.LoadBalancers[0] = infrav1.LoadBalancerSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "lb-id-ref",
					},
				}
				// Set both NetworkReadyCondition and LoadBalancerReadyCondition so controller proceeds to get hostname
				powerVSCluster.Status.Conditions = []metav1.Condition{
					{
						Type:   infrav1.NetworkReadyCondition,
						Status: metav1.ConditionTrue,
						Reason: infrav1.NetworkReadyReason,
					},
					{
						Type:   infrav1.LoadBalancerReadyCondition,
						Status: metav1.ConditionTrue,
						Reason: infrav1.VPCLoadBalancerReadyReason,
					},
				}
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: powerVSCluster,
					Cluster: &clusterv1.Cluster{
						Spec: clusterv1.ClusterSpec{
							ClusterNetwork: clusterv1.ClusterNetwork{
								APIServerPort: 6443,
							},
						},
					},
				}
				clusterScope.IBMPowerVSClient = getMockPowerVS(t)
				clusterScope.ResourceClient = getMockResourceController(t)
				clusterScope.ResourceManagerClient = getMockResourceManager(t)
				clusterScope.TransitGatewayClient = getMockTransitGateway(t)

				mockVPC := getMockVPC(t)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, nil, errors.New("failed to get loadbalancer"))
				clusterScope.IBMVPCClient = mockVPC

				return clusterScope
			},
			expectedError: fmt.Errorf("failed to fetch public loadbalancer: %w", fmt.Errorf("failed to fetch referenced load balancer (%s) details: %w", "lb-id-ref", errors.New("failed to get loadbalancer"))),
			clusterStatus: false,
		},
		{
			name: "When reconcile is successful",
			powervsClusterScope: func() *powervsscope.ClusterScope {
				powerVSCluster := getPowerVSClusterWithSpecAndStatus()
				// Add hostname to status so GetPublicLoadBalancerHostName returns it
				powerVSCluster.Status.LoadBalancers[0].Hostname = "hostname"
				// Set NetworkReadyCondition and LoadBalancerReadyCondition so controller proceeds
				powerVSCluster.Status.Conditions = []metav1.Condition{
					{
						Type:   infrav1.NetworkReadyCondition,
						Status: metav1.ConditionTrue,
						Reason: infrav1.NetworkReadyReason,
					},
					{
						Type:   infrav1.LoadBalancerReadyCondition,
						Status: metav1.ConditionTrue,
						Reason: infrav1.VPCLoadBalancerReadyReason,
					},
				}
				clusterScope := &powervsscope.ClusterScope{
					Cluster:           &clusterv1.Cluster{},
					IBMPowerVSCluster: powerVSCluster,
				}
				clusterScope.IBMPowerVSClient = getMockPowerVS(t)
				clusterScope.ResourceClient = getMockResourceController(t)
				clusterScope.ResourceManagerClient = getMockResourceManager(t)
				clusterScope.TransitGatewayClient = getMockTransitGateway(t)
				clusterScope.IBMVPCClient = getMockVPC(t)

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
			res, err := reconciler.reconcile(ctx, powerVSClusterScope)
			if tc.expectedError != nil {
				if errAggregate, ok := err.(kerrors.Aggregate); ok {
					for _, e := range errAggregate.Errors() {
						g.Expect(tc.expectedError.Error()).To(ContainSubstring(e.Error()))
					}
				}
			} else {
				g.Expect(err).To(BeNil())
			}
			g.Expect(res).To(Equal(tc.expectedResult))
			g.Expect(ptr.Deref(powerVSClusterScope.IBMPowerVSCluster.Status.Initialization.Provisioned, false)).To(Equal(tc.clusterStatus))
			g.Expect(powerVSClusterScope.IBMPowerVSCluster.Finalizers).To(ContainElement(infrav1.IBMPowerVSClusterFinalizer))
			if len(tc.conditions) > 1 {
				ignoreLastTransitionTime := cmp.Transformer("", func(metav1.Time) metav1.Time {
					return metav1.Time{}
				})
				// TODO: Update tests to use GetConditions()
				g.Expect(powerVSClusterScope.IBMPowerVSCluster.GetV1Beta1Conditions()).To(BeComparableTo(tc.conditions, ignoreLastTransitionTime))
			}
		})
	}
}

func TestIBMPowerVSClusterReconciler_delete(t *testing.T) {
	var (
		reconciler          IBMPowerVSClusterReconciler
		clusterScope        *powervsscope.ClusterScope
		mockPowerVS         *powervsmock.MockPowerVS
		mockTransitGateway  *tgmock.MockTransitGateway
		mockVpc             *vpcmock.MockVpc
		mockResourceClient  *resourceclientmock.MockResourceController
		powervsClusterScope func() *powervsscope.ClusterScope
	)
	reconciler = IBMPowerVSClusterReconciler{
		Client: testEnv.Client,
	}
	powervsClusterScope = func() *powervsscope.ClusterScope {
		return &powervsscope.ClusterScope{
			IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
				TypeMeta: metav1.TypeMeta{
					Kind:       "IBMPowerVSCluster",
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        "capi-powervs-cluster",
					Annotations: map[string]string{"powervs.cluster.x-k8s.io/create-infra": "true"},
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: clusterv1.GroupVersion.String(),
							Kind:       "Cluster",
							Name:       "capi-test",
							UID:        "1",
						}},
				},
				Spec: infrav1.IBMPowerVSClusterSpec{
					Topology: infrav1.PowerVSVirtualIPTopology,
				},
				Status: infrav1.IBMPowerVSClusterStatus{
					Workspace: infrav1.ResourceReferenceV1Beta3{
						ID: "serviceInstanceID",
					},
				},
			},
			Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects().Build(),
		}
	}

	t.Run("Reconciling delete IBMPowerVSCluster", func(t *testing.T) {
		t.Run("Should reconcile successfully if no descendants are found", func(t *testing.T) {
			g := NewWithT(t)
			clusterScope = &powervsscope.ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					TypeMeta: metav1.TypeMeta{
						Kind:       "IBMPowerVSCluster",
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "capi-powervs-cluster",
					},
					Spec: infrav1.IBMPowerVSClusterSpec{
						Topology: infrav1.PowerVSVirtualIPTopology,
						Workspace: infrav1.WorkspaceSource{
							Type: infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{
								ID: "service-instance-1",
							},
						},
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
			clusterScope = &powervsscope.ClusterScope{
				IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
					TypeMeta: metav1.TypeMeta{
						Kind:       "IBMPowerVSCluster",
						APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Name: "capi-powervs-cluster",
					},
					Spec: infrav1.IBMPowerVSClusterSpec{
						Topology: infrav1.PowerVSVirtualIPTopology,
						Workspace: infrav1.WorkspaceSource{
							Type: infrav1.SourceTypeReference,
							Reference: infrav1.ResourceIdentifier{
								ID: "service-instance-1",
							},
						},
					},
				},
				Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects().Build(),
			}
			powervsImage1 := &infrav1.IBMPowerVSImage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-image",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: infrav1.GroupVersion.String(),
							Kind:       "IBMPowerVSCluster",
							Name:       "capi-powervs-cluster",
							UID:        "1",
						},
					},
					Labels: map[string]string{clusterv1.ClusterNameLabel: "capi-powervs-cluster"},
				},
				Spec: infrav1.IBMPowerVSImageSpec{
					ClusterName: "capi-powervs-cluster",
					Object:      ptr.To("capi-image.ova.gz"),
					Region:      ptr.To("us-south"),
					Bucket:      ptr.To("capi-bucket"),
				},
			}
			powervsImage2 := &infrav1.IBMPowerVSImage{
				ObjectMeta: metav1.ObjectMeta{
					Name: "capi-image2",
					OwnerReferences: []metav1.OwnerReference{
						{
							APIVersion: infrav1.GroupVersion.String(),
							Kind:       "IBMPowerVSCluster",
							Name:       "capi-powervs-cluster",
							UID:        "1",
						},
					},
					Labels: map[string]string{clusterv1.ClusterNameLabel: "capi-powervs-cluster"},
				},
				Spec: infrav1.IBMPowerVSImageSpec{
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

	t.Run("When delete TransitGateway returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Spec.TransitGateway = infrav1.TransitGatewaySource{
			Type: infrav1.SourceTypeProvision,
			PowerVSConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeProvision,
			},
			VPCConnection: infrav1.TransitGatewayConnectionSource{
				Type: infrav1.SourceTypeProvision,
			},
		}
		clusterScope.IBMPowerVSCluster.Status.TransitGateway = infrav1.TransitGatewayStatus{
			ID: "transitgatewayID",
			PowerVSConnection: infrav1.ResourceConnectionStatus{
				ID: "connectionID",
			},
			VPCConnection: infrav1.ResourceConnectionStatus{
				ID: "connectionID",
			},
		}
		tgw := &tgapiv1.TransitGateway{
			Name:   ptr.To("transitGateway"),
			ID:     ptr.To("transitGatewayID"),
			Status: ptr.To(string(infrav1.TransitGatewayStateAvailable))}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		clusterScope.ResourceClient = mockResourceClient
		mockTransitGateway = tgmock.NewMockTransitGateway(gomock.NewController(t))
		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, nil, nil)
		mockTransitGateway.EXPECT().GetTransitGatewayConnection(gomock.Any()).Return(nil, &core.DetailedResponse{StatusCode: 404}, errors.New("connection not found")).Times(2)
		mockTransitGateway.EXPECT().DeleteTransitGateway(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete transit gateway"))
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("When delete TransitGateway returns requeue as true", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Status.TransitGateway = infrav1.TransitGatewayStatus{
			ID: "transitgatewayID",
			PowerVSConnection: infrav1.ResourceConnectionStatus{
				ID: "connectionID",
			},
			VPCConnection: infrav1.ResourceConnectionStatus{
				ID: "connectionID",
			},
		}
		tgw := &tgapiv1.TransitGateway{
			Name:   ptr.To("transitGateway"),
			ID:     ptr.To("transitGatewayID"),
			Status: ptr.To(string(infrav1.TransitGatewayStateDeletePending))}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		clusterScope.ResourceClient = mockResourceClient
		mockTransitGateway = tgmock.NewMockTransitGateway(gomock.NewController(t))
		mockTransitGateway.EXPECT().GetTransitGateway(gomock.Any()).Return(tgw, nil, nil)
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(BeNil())
		g.Expect(result).To(Equal(ctrl.Result{RequeueAfter: time.Minute}))
	})

	t.Run("When delete LoadBalancer returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type: infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{
					Name: "lb",
				},
			},
		}
		clusterScope.IBMPowerVSCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{
			{
				Name: "lb",
				ID:   "lb-id",
			},
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		clusterScope.ResourceClient = mockResourceClient
		mockTransitGateway = tgmock.NewMockTransitGateway(gomock.NewController(t))
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateActive)),
		}, nil, nil)
		mockVpc.EXPECT().DeleteLoadBalancer(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete load balancer"))
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("When delete LoadBalancer returns requeue as true", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Spec.LoadBalancers = []infrav1.LoadBalancerSource{
			{
				Type: infrav1.SourceTypeProvision,
				Provision: infrav1.LoadBalancerProvision{
					Name: "lb",
				},
			},
		}
		clusterScope.IBMPowerVSCluster.Status.LoadBalancers = []infrav1.LoadBalancerStatus{
			{
				Name: "lb",
				ID:   "lb-id",
			},
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		clusterScope.ResourceClient = mockResourceClient
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
			ID:                 ptr.To("lb-id"),
			Name:               ptr.To("lb"),
			ProvisioningStatus: ptr.To(string(infrav1.LoadBalancerStateDeletePending)),
		}, nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(BeNil())
		g.Expect(result).To(Equal(ctrl.Result{RequeueAfter: time.Minute}))
	})

	t.Run("When delete VPC security group returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Status.VPCSecurityGroups = map[string]infrav1.VPCSecurityGroupStatus{
			"sc": {
				ID:                ptr.To("sc-id"),
				ControllerCreated: ptr.To(true),
			},
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		clusterScope.ResourceClient = mockResourceClient
		mockTransitGateway = tgmock.NewMockTransitGateway(gomock.NewController(t))
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		mockVpc.EXPECT().GetSecurityGroup(gomock.Any()).Return(&vpcv1.SecurityGroup{
			ID:   ptr.To("sc-id"),
			Name: ptr.To("sc"),
		}, nil, nil)
		mockVpc.EXPECT().DeleteSecurityGroup(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete VPC security group"))
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("When delete VPC subnet returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Status.VPCSubnets = []infrav1.VPCSubnetStatus{
			{
				ID:   "subnet1",
				Name: "subnet1",
			},
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		clusterScope.ResourceClient = mockResourceClient
		mockTransitGateway = tgmock.NewMockTransitGateway(gomock.NewController(t))
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		mockVpc.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
		mockVpc.EXPECT().DeleteSubnet(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete VPC subnet"))
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("When delete VPC subnet returns requeue as true", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Status.VPCSubnets = []infrav1.VPCSubnetStatus{
			{
				ID:   "subnet1",
				Name: "subnet1",
			},
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		clusterScope.ResourceClient = mockResourceClient
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		mockVpc.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{Name: ptr.To("subnet1"), Status: ptr.To(string(infrav1.VPCSubnetStateDeleting))}, nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(BeNil())
		g.Expect(result).To(Equal(ctrl.Result{RequeueAfter: 15 * time.Second}))
	})

	t.Run("When delete VPC returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Spec.VPC = infrav1.VPCSource{
			Type: infrav1.SourceTypeProvision,
		}
		clusterScope.IBMPowerVSCluster.Status.VPC = infrav1.VPCStatus{
			ID:   "vpcid",
			Name: "vpcName",
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		clusterScope.ResourceClient = mockResourceClient
		mockTransitGateway = tgmock.NewMockTransitGateway(gomock.NewController(t))
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		mockVpc.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{ID: ptr.To("vpcid"), Name: ptr.To("vpcName"), Status: ptr.To("active")}, nil, nil)
		mockVpc.EXPECT().DeleteVPC(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete VPC"))
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("When delete VPC returns requeue as true", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Spec.VPC = infrav1.VPCSource{
			Type: infrav1.SourceTypeProvision,
		}
		clusterScope.IBMPowerVSCluster.Status.VPC = infrav1.VPCStatus{
			ID:   "vpcid",
			Name: "vpcName",
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		clusterScope.ResourceClient = mockResourceClient
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		mockVpc.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{ID: ptr.To("vpcid"), Name: ptr.To("vpcName"), Status: ptr.To(string(infrav1.VPCStateDeleting))}, nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(BeNil())
		g.Expect(result).To(Equal(ctrl.Result{RequeueAfter: 15 * time.Second}))
	})

	t.Run("When delete DHCP returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		// Set Network.Type to Provision so DHCP deletion is attempted
		clusterScope.IBMPowerVSCluster.Spec.Network = infrav1.NetworkSource{
			Type: infrav1.SourceTypeProvision,
			Provision: infrav1.NetworkProvisionConfig{
				DHCPServer: infrav1.DHCPServer{
					Name: "dhcp-server",
				},
			},
		}
		// Set Workspace.Type to Reference so workspace deletion doesn't cascade
		clusterScope.IBMPowerVSCluster.Spec.Workspace = infrav1.WorkspaceSource{
			Type: infrav1.SourceTypeReference,
			Reference: infrav1.ResourceIdentifier{
				ID: "serviceInstanceID",
			},
		}
		clusterScope.IBMPowerVSCluster.Status = infrav1.IBMPowerVSClusterStatus{
			Workspace: infrav1.ResourceReferenceV1Beta3{
				ID: "serviceInstanceID",
			},
			Network: infrav1.NetworkStatus{
				DHCPServer: infrav1.ResourceReferenceV1Beta3{
					ID: "DHCPServerID",
				},
			},
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		mockPowerVS.EXPECT().GetDHCPServer(gomock.Any()).Return(&models.DHCPServerDetail{
			ID:     ptr.To("dhcpID"),
			Status: ptr.To(string(infrav1.DHCPServerStateActive)),
		}, nil)
		mockPowerVS.EXPECT().DeleteDHCPServer(gomock.Any()).Return(errors.New("failed to delete DHCP server"))
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		clusterScope.ResourceClient = mockResourceClient
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("When delete Workspace returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Spec.Workspace.Type = infrav1.SourceTypeProvision
		clusterScope.IBMPowerVSCluster.Status = infrav1.IBMPowerVSClusterStatus{
			Workspace: infrav1.ResourceReferenceV1Beta3{
				ID: "serviceInstanceID",
			},
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("serviceInstanceName"),
			ID:    ptr.To("serviceInstanceID"),
			State: ptr.To("active"),
			CRN:   ptr.To("powervs_crn"),
		}, nil, nil)
		mockResourceClient.EXPECT().DeleteResourceInstance(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete service instance"))
		clusterScope.ResourceClient = mockResourceClient
		mockTransitGateway = tgmock.NewMockTransitGateway(gomock.NewController(t))
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("When delete Workspace returns requeue as true", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Spec.Workspace.Type = infrav1.SourceTypeProvision
		clusterScope.IBMPowerVSCluster.Status = infrav1.IBMPowerVSClusterStatus{
			Workspace: infrav1.ResourceReferenceV1Beta3{
				ID: "serviceInstanceID",
			},
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("serviceInstanceName"),
			ID:    ptr.To("serviceInstanceID"),
			State: ptr.To("active"),
			CRN:   ptr.To("powervs_crn"),
		}, nil, nil)
		mockResourceClient.EXPECT().DeleteResourceInstance(gomock.Any()).Return(&core.DetailedResponse{}, nil)
		clusterScope.ResourceClient = mockResourceClient
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(BeNil())
		g.Expect(result).To(Equal(ctrl.Result{RequeueAfter: time.Minute}))
	})

	t.Run("When delete COSInstance returns error", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Status.COSInstance = &infrav1.ResourceReference{
			ID:                ptr.To("CosInstanceID"),
			ControllerCreated: ptr.To(true),
		}
		clusterScope.IBMPowerVSCluster.Spec = infrav1.IBMPowerVSClusterSpec{
			Topology: infrav1.PowerVSLoadBalancerTopology,
			Workspace: infrav1.WorkspaceSource{
				Type: infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{
					ID: "service-instance-1",
				},
			},
			LoadBalancers: []infrav1.LoadBalancerSource{},
			Ignition:      &infrav1.Ignition{Version: "3.4"},
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		mockResourceClient.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{
			Name:  ptr.To("COSInstanceName"),
			ID:    ptr.To("COSInstanceID"),
			State: ptr.To("active"),
		}, nil, nil)
		mockResourceClient.EXPECT().DeleteResourceInstance(gomock.Any()).Return(&core.DetailedResponse{}, errors.New("failed to delete COS service instance"))
		clusterScope.ResourceClient = mockResourceClient
		mockTransitGateway = tgmock.NewMockTransitGateway(gomock.NewController(t))
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(Not(BeNil()))
		g.Expect(result.RequeueAfter).To(BeZero())
	})

	t.Run("When reconcile delete is successful", func(t *testing.T) {
		g := NewWithT(t)
		clusterScope = powervsClusterScope()
		clusterScope.IBMPowerVSCluster.Spec.Topology = infrav1.PowerVSLoadBalancerTopology
		clusterScope.IBMPowerVSCluster.Status = infrav1.IBMPowerVSClusterStatus{
			Workspace: infrav1.ResourceReferenceV1Beta3{
				ID: "serviceInstanceID",
			},
		}
		clusterScope.IBMPowerVSCluster.Spec = infrav1.IBMPowerVSClusterSpec{
			Topology: infrav1.PowerVSLoadBalancerTopology,
			Workspace: infrav1.WorkspaceSource{
				Type: infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{
					ID: "service-instance-1",
				},
			},
			LoadBalancers: []infrav1.LoadBalancerSource{},
			Ignition:      &infrav1.Ignition{Version: "3.4"},
		}
		mockPowerVS = powervsmock.NewMockPowerVS(gomock.NewController(t))
		mockPowerVS.EXPECT().WithClients(gomock.Any())
		clusterScope.IBMPowerVSClient = mockPowerVS
		mockResourceClient = resourceclientmock.NewMockResourceController(gomock.NewController(t))
		clusterScope.ResourceClient = mockResourceClient
		mockTransitGateway = tgmock.NewMockTransitGateway(gomock.NewController(t))
		clusterScope.TransitGatewayClient = mockTransitGateway
		mockVpc = vpcmock.NewMockVpc(gomock.NewController(t))
		mockVpc.EXPECT().GetLoadBalancerByName(gomock.Any()).Return(nil, nil)
		clusterScope.IBMVPCClient = mockVpc
		result, err := reconciler.reconcileDelete(ctx, clusterScope)
		g.Expect(err).To(BeNil())
		g.Expect(result.RequeueAfter).To(BeZero())
	})
}

func TestReconcileVPCResources(t *testing.T) {
	testCases := []struct {
		name                    string
		powerVSClusterScopeFunc func() *powervsscope.ClusterScope
		reconcileResult         reconcileResult
		conditions              clusterv1.Conditions
	}{
		{
			name: "when ReconcileVPC returns error",
			powerVSClusterScopeFunc: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							VPC: infrav1.VPCSource{
								Type:      infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{Name: "vpc-name"},
							},
						},
					},
				}
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPCByName(gomock.Any()).Return(nil, errors.New("vpc not found"))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: errors.New("vpc not found"),
			},
			conditions: clusterv1.Conditions{
				clusterv1.Condition{
					Type:               infrav1.VPCReadyCondition,
					Status:             "False",
					Severity:           clusterv1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1.VPCReconciliationFailedReason,
					Message:            "failed to get referenced VPC: vpc not found",
				},
			},
		},
		{
			name: "when ReconcileVPC returns requeue as true",
			powerVSClusterScopeFunc: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							VPC: infrav1.VPCSource{Type: infrav1.SourceTypeProvision},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
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
					RequeueAfter: 20 * time.Second,
				},
			},
		},
		{
			name: "when Reconciling VPC subnets returns error",
			powerVSClusterScopeFunc: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSLoadBalancerTopology,
							VPC:      infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "us-south"},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
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

			conditions: clusterv1.Conditions{
				getVPCReadyCondition(),
				clusterv1.Condition{
					Type:               infrav1.VPCSubnetReadyCondition,
					Status:             "False",
					Severity:           clusterv1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1.VPCSubnetReconciliationFailedReason,
					Message:            "failed resolving subnet -subnet-us-south-1: failed checking subnet presence by name: vpc subnet not found",
				},
			},
		},
		{
			name: "when Reconciling VPC subnets returns requeue as true",
			powerVSClusterScopeFunc: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology:      infrav1.PowerVSLoadBalancerTopology,
							ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "rg-id"}},
							VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "us-south"},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
						},
					},
				}
				vpcZones, _ := regionUtil.VPCZonesForVPCRegion("us-south")
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetVPCSubnetByName(gomock.Any()).Return(nil, nil).Times(len(vpcZones))
				mockVPC.EXPECT().CreateSubnet(gomock.Any()).Return(&vpcv1.Subnet{Status: ptr.To("active")}, nil, nil).Times(len(vpcZones))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				Result: reconcile.Result{
					RequeueAfter: 20 * time.Second,
				},
			},
			conditions: clusterv1.Conditions{
				getVPCReadyCondition(),
			},
		},
		{
			name: "when Reconciling VPC security group returns error",
			powerVSClusterScopeFunc: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSLoadBalancerTopology,
							VPC:      infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "us-south"},
							VPCSubnets: []infrav1.VPCSubnetSource{
								{
									Type:      infrav1.SourceTypeReference,
									Reference: infrav1.ResourceIdentifier{ID: "subnet-id", Name: "subnet1"},
								},
							},
							VPCSecurityGroups: []infrav1.VPCSecurityGroup{
								{
									Name: ptr.To("security-group"),
								},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
							VPCSubnets: []infrav1.VPCSubnetStatus{
								{ID: "subnet-id", Name: "subnet1"},
							},
						},
					},
				}
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{ID: ptr.To("subnet-id"), Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSecurityGroupByName(gomock.Any()).Return(nil, errors.New("vpc security group not found"))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: errors.New("failed to validate existing security group: vpc security group not found"),
			},

			conditions: clusterv1.Conditions{
				getVPCReadyCondition(),
				clusterv1.Condition{
					Type:               infrav1.VPCSecurityGroupReadyCondition,
					Status:             "False",
					Severity:           clusterv1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1.VPCSecurityGroupReconciliationFailedReason,
					Message:            "failed to validate existing security group: vpc security group not found",
				},
				getVPCSubnetReadyCondition(),
			},
		},
		{
			name: "when Reconciling LoadBalancer returns error",
			powerVSClusterScopeFunc: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSLoadBalancerTopology,
							VPC:      infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "us-south"},
							VPCSubnets: []infrav1.VPCSubnetSource{
								{
									Type:      infrav1.SourceTypeReference,
									Reference: infrav1.ResourceIdentifier{ID: "subnet-id", Name: "subnet1"},
								},
							},
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type:      infrav1.SourceTypeReference,
									Reference: infrav1.ResourceIdentifier{ID: "lb-id", Name: "lb"},
								},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
							VPCSubnets: []infrav1.VPCSubnetStatus{
								{ID: "subnet-id", Name: "subnet1"},
							},
						},
					},
				}
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{ID: ptr.To("subnet-id"), Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(nil, nil, errors.New("load balancer not found"))
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: errors.New("load balancer not found"),
			},

			conditions: clusterv1.Conditions{
				clusterv1.Condition{
					Type:               infrav1.LoadBalancerReadyCondition,
					Status:             "False",
					Severity:           clusterv1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1.LoadBalancerReconciliationFailedReason,
					Message:            "failed to fetch referenced load balancer details: load balancer not found",
				},
				getVPCReadyCondition(),
				getVPCSGReadyCondition(),
				getVPCSubnetReadyCondition(),
			},
		},
		{
			name: "when Reconciling LoadBalancer returns with loadbalancer status as ready",
			powerVSClusterScopeFunc: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSLoadBalancerTopology,
							VPC:      infrav1.VPCSource{Type: infrav1.SourceTypeReference, Region: "us-south"},
							VPCSubnets: []infrav1.VPCSubnetSource{
								{
									Type:      infrav1.SourceTypeReference,
									Reference: infrav1.ResourceIdentifier{ID: "subnet-id", Name: "subnet1"},
								},
							},
							LoadBalancers: []infrav1.LoadBalancerSource{
								{
									Type:      infrav1.SourceTypeReference,
									Reference: infrav1.ResourceIdentifier{ID: "lb-id", Name: "lb"},
								},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
							VPCSubnets: []infrav1.VPCSubnetStatus{
								{ID: "subnet-id", Name: "subnet1"},
							},
						},
					},
				}
				mockVPC := vpcmock.NewMockVpc(gomock.NewController(t))
				mockVPC.EXPECT().GetVPC(gomock.Any()).Return(&vpcv1.VPC{Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{ID: ptr.To("subnet-id"), Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
				mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
					ID:                 ptr.To("lb-id"),
					Name:               ptr.To("lb"),
					ProvisioningStatus: ptr.To("active"),
				}, nil, nil)
				clusterScope.IBMVPCClient = mockVPC
				return clusterScope
			},
			conditions: clusterv1.Conditions{
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
			reconciler.reconcileVPCResources(ctx, clusterScope, pvsCluster, ch, wg)
			wg.Wait()
			close(ch)
			result := <-ch
			g.Expect(result.Result).To(Equal(tc.reconcileResult.Result))
			if tc.reconcileResult.error != nil {
				g.Expect(result).To(MatchError(ContainSubstring(tc.reconcileResult.Error())))
			} else {
				g.Expect(result.error).To(BeNil())
			}
			ignoreLastTransitionTime := cmp.Transformer("", func(metav1.Time) metav1.Time {
				return metav1.Time{}
			})
			g.Expect(pvsCluster.cluster.GetV1Beta1Conditions()).To(BeComparableTo(tc.conditions, ignoreLastTransitionTime))
		})
	}
}

func TestReconcilePowerVSResources(t *testing.T) {
	testCases := []struct {
		name                    string
		powerVSClusterScopeFunc func() *powervsscope.ClusterScope
		reconcileResult         reconcileResult
		conditions              clusterv1.Conditions
	}{
		{
			name: "When Reconciling PowerVS service instance returns error",
			powerVSClusterScopeFunc: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Status: infrav1.IBMPowerVSClusterStatus{
							Workspace: infrav1.ResourceReferenceV1Beta3{
								ID: "serviceInstanceID",
							},
						},
					},
				}
				mockResourceController := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(nil, nil, errors.New("error getting resource instance"))
				clusterScope.ResourceClient = mockResourceController
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: errors.New("error getting resource instance"),
			},

			conditions: clusterv1.Conditions{
				clusterv1.Condition{
					Type:               infrav1.ServiceInstanceReadyV1Beta2Condition,
					Status:             "False",
					Severity:           clusterv1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1.ServiceInstanceReconciliationFailedV1Beta2Reason,
					Message:            "failed to fetch workspace (id: serviceInstanceID) details: error getting resource instance",
				},
			},
		},
		{
			name: "When Reconciling PowerVS service instance returns requeue as true",
			powerVSClusterScopeFunc: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Status: infrav1.IBMPowerVSClusterStatus{
							Workspace: infrav1.ResourceReferenceV1Beta3{
								ID: "serviceInstanceID",
							},
						},
					},
				}
				mockResourceController := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{State: ptr.To(string(infrav1.WorkspaceStateProvisioning)), Name: ptr.To("serviceInstanceName")}, nil, nil)
				clusterScope.ResourceClient = mockResourceController
				return clusterScope
			},
			reconcileResult: reconcileResult{
				Result: reconcile.Result{
					RequeueAfter: 20 * time.Second,
				},
			},
		},
		{
			name: "When Reconciling network returns error",
			powerVSClusterScopeFunc: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSVirtualIPTopology,
							Workspace: infrav1.WorkspaceSource{
								Type: infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{
									ID: "serviceInstanceID",
								},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							Network: infrav1.NetworkStatus{
								ID: "NetworkID",
							},
							Workspace: infrav1.ResourceReferenceV1Beta3{
								ID: "serviceInstanceID",
							},
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(nil, errors.New("error getting network"))
				mockPowerVS.EXPECT().WithClients(gomock.Any())
				mockResourceController := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{State: ptr.To(string(infrav1.WorkspaceStateActive)), Name: ptr.To("serviceInstanceName")}, nil, nil)
				clusterScope.ResourceClient = mockResourceController
				clusterScope.IBMPowerVSClient = mockPowerVS
				return clusterScope
			},
			reconcileResult: reconcileResult{
				error: errors.New("error getting network"),
			},
			conditions: clusterv1.Conditions{
				clusterv1.Condition{
					Type:               infrav1.NetworkReadyCondition,
					Status:             "False",
					Severity:           clusterv1.ConditionSeverityError,
					LastTransitionTime: metav1.Time{},
					Reason:             infrav1.NetworkReconciliationFailedReason,
					Message:            "failed to fetch network by ID: error getting network",
				},
				getWorkspaceReadyCondition(),
			},
		},
		{
			name: "When reconcile network returns with DHCP server in active state",
			powerVSClusterScopeFunc: func() *powervsscope.ClusterScope {
				clusterScope := &powervsscope.ClusterScope{
					IBMPowerVSCluster: &infrav1.IBMPowerVSCluster{
						Spec: infrav1.IBMPowerVSClusterSpec{
							Topology: infrav1.PowerVSVirtualIPTopology,
							Workspace: infrav1.WorkspaceSource{
								Type: infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{
									ID: "serviceInstanceID",
								},
							},
							Network: infrav1.NetworkSource{
								Type: infrav1.SourceTypeReference,
								Reference: infrav1.ResourceIdentifier{
									ID: "netID",
								},
							},
						},
						Status: infrav1.IBMPowerVSClusterStatus{
							Network: infrav1.NetworkStatus{
								ID: "netID",
							},
							Workspace: infrav1.ResourceReferenceV1Beta3{
								ID: "serviceInstanceID",
							},
						},
					},
				}
				mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
				mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(&models.Network{NetworkID: ptr.To("netID")}, nil)
				mockPowerVS.EXPECT().WithClients(gomock.Any())
				mockResourceController := resourceclientmock.NewMockResourceController(gomock.NewController(t))
				mockResourceController.EXPECT().GetResourceInstance(gomock.Any()).Return(&resourcecontrollerv2.ResourceInstance{State: ptr.To(string(infrav1.WorkspaceStateActive)), Name: ptr.To("serviceInstanceName")}, nil, nil)
				clusterScope.ResourceClient = mockResourceController
				clusterScope.IBMPowerVSClient = mockPowerVS
				return clusterScope
			},
			conditions: clusterv1.Conditions{
				getWorkspaceReadyCondition(),
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
			reconciler.reconcilePowerVSResources(ctx, clusterScope, pvsCluster, ch, wg)
			wg.Wait()
			close(ch)
			result := <-ch
			g.Expect(result.Result).To(Equal(tc.reconcileResult.Result))
			if tc.reconcileResult.error != nil {
				g.Expect(result).To(MatchError(ContainSubstring(tc.reconcileResult.Error())))
			} else {
				g.Expect(result.error).To(BeNil())
			}
			ignoreLastTransitionTime := cmp.Transformer("", func(metav1.Time) metav1.Time {
				return metav1.Time{}
			})
			g.Expect(pvsCluster.cluster.GetV1Beta1Conditions()).To(BeComparableTo(tc.conditions, ignoreLastTransitionTime))
		})
	}
}

func getWorkspaceReadyCondition() clusterv1.Condition {
	return clusterv1.Condition{
		Type:   infrav1.ServiceInstanceReadyCondition,
		Status: "True",
	}
}

func getVPCReadyCondition() clusterv1.Condition {
	return clusterv1.Condition{
		Type:   infrav1.VPCReadyCondition,
		Status: "True",
	}
}

func getVPCSubnetReadyCondition() clusterv1.Condition {
	return clusterv1.Condition{
		Type:   infrav1.VPCSubnetReadyCondition,
		Status: "True",
	}
}

func getVPCSGReadyCondition() clusterv1.Condition {
	return clusterv1.Condition{
		Type:   infrav1.VPCSecurityGroupReadyCondition,
		Status: "True",
	}
}

func getVPCLBReadyCondition() clusterv1.Condition {
	return clusterv1.Condition{
		Type:   infrav1.LoadBalancerReadyCondition,
		Status: "True",
	}
}

func getTGReadyCondition() clusterv1.Condition {
	return clusterv1.Condition{
		Type:   infrav1.TransitGatewayReadyCondition,
		Status: "True",
	}
}

func getPowerVSClusterWithSpecAndStatus() *infrav1.IBMPowerVSCluster {
	return &infrav1.IBMPowerVSCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "capi-powervs-cluster",
			Finalizers:  []string{infrav1.IBMPowerVSClusterFinalizer},
			Annotations: map[string]string{infrav1.CreateInfrastructureAnnotation: "true"},
		},
		Spec: infrav1.IBMPowerVSClusterSpec{
			Topology:      infrav1.PowerVSLoadBalancerTopology,
			Zone:          "dal10",
			ResourceGroup: infrav1.ResourceGroupSource{Type: infrav1.SourceTypeReference, Reference: infrav1.ResourceIdentifier{ID: "rg-id"}},
			VPC:           infrav1.VPCSource{Type: infrav1.SourceTypeProvision, Region: "us-south"},
			VPCSubnets: []infrav1.VPCSubnetSource{
				{
					Type:      infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{ID: "subnet-id", Name: "subnet1"},
				},
			},
			LoadBalancers: []infrav1.LoadBalancerSource{
				{
					Type:      infrav1.SourceTypeProvision,
					Provision: infrav1.LoadBalancerProvision{Name: "capi-powervs-cluster-lb-public"},
				},
			},
			TransitGateway: infrav1.TransitGatewaySource{
				Type: infrav1.SourceTypeReference,
				Reference: infrav1.ResourceIdentifier{
					ID: "transitGatewayID",
				},
				PowerVSConnection: infrav1.TransitGatewayConnectionSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "powervs-conn-id",
					},
				},
				VPCConnection: infrav1.TransitGatewayConnectionSource{
					Type: infrav1.SourceTypeReference,
					Reference: infrav1.ResourceIdentifier{
						ID: "vpc-conn-id",
					},
				},
			},
		},
		Status: infrav1.IBMPowerVSClusterStatus{
			Workspace: infrav1.ResourceReferenceV1Beta3{
				ID: "serviceInstanceID",
			},
			Network: infrav1.NetworkStatus{
				ID: "NetworkID",
			},
			VPC: infrav1.VPCStatus{ID: "vpcID", Name: "vpcName"},
			VPCSubnets: []infrav1.VPCSubnetStatus{
				{ID: "subnet-id", Name: "subnet1"},
			},
			LoadBalancers: []infrav1.LoadBalancerStatus{
				{ID: "lb-id", Name: "capi-powervs-cluster-lb-public"},
			},
			TransitGateway: infrav1.TransitGatewayStatus{
				ID: "transitGatewayID",
			},
		},
	}
}

func getMockPowerVS(t *testing.T) *powervsmock.MockPowerVS {
	t.Helper()
	mockPowerVS := powervsmock.NewMockPowerVS(gomock.NewController(t))
	mockPowerVS.EXPECT().GetDatatcenterDetails(gomock.Any()).Return(&models.Datacenter{
		Capabilities: map[string]bool{"power-edge-router": true},
	}, nil)
	network := &models.Network{NetworkID: ptr.To("netID")}
	mockPowerVS.EXPECT().GetNetworkByID(gomock.Any()).Return(network, nil)
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
	mockVPC.EXPECT().GetSubnet(gomock.Any()).Return(&vpcv1.Subnet{ID: ptr.To("subnet-id"), Name: ptr.To("subnet1"), Status: ptr.To("active")}, nil, nil)
	mockVPC.EXPECT().GetLoadBalancer(gomock.Any()).Return(&vpcv1.LoadBalancer{
		ID:                 ptr.To("lb-id"),
		Name:               ptr.To("capi-powervs-cluster-lb-public"),
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
		Status: ptr.To(string(infrav1.TransitGatewayStateAvailable)),
	}, nil, nil)
	mockTransitGateway.EXPECT().ListTransitGatewayConnections(gomock.Any()).Return(&tgapiv1.TransitGatewayConnectionCollection{
		Connections: []tgapiv1.TransitGatewayConnectionCust{
			{
				ID:          ptr.To("vpc-conn-id"),
				Name:        ptr.To("vpc_connection"),
				NetworkID:   ptr.To("vpc_crn"),
				NetworkType: ptr.To("vpc"),
				Status:      ptr.To(string(infrav1.TransitGatewayConnectionStateAttached)),
			},
			{
				ID:          ptr.To("powervs-conn-id"),
				Name:        ptr.To("powervs_connection"),
				NetworkID:   ptr.To("powervs_crn"),
				NetworkType: ptr.To("power_virtual_server"),
				Status:      ptr.To(string(infrav1.TransitGatewayConnectionStateAttached)),
			},
		},
	}, nil, nil)
	return mockTransitGateway
}

func getMockResourceManager(t *testing.T) *resourcemanagermock.MockResourceManager {
	t.Helper()
	mockResourceManager := resourcemanagermock.NewMockResourceManager(gomock.NewController(t))
	mockResourceManager.EXPECT().GetResourceGroup(gomock.Any()).Return(&resourcemanagerv2.ResourceGroup{
		ID:   ptr.To("rg-id"),
		Name: ptr.To("resource-group-name"),
	}, nil, nil).AnyTimes()
	return mockResourceManager
}

func createCluster(g *WithT, powervsCluster *infrav1.IBMPowerVSCluster, namespace string) {
	if powervsCluster != nil {
		powervsCluster.Namespace = namespace
		g.Expect(testEnv.Create(ctx, powervsCluster)).To(Succeed())
		g.Eventually(func() bool {
			cluster := &infrav1.IBMPowerVSCluster{}
			key := client.ObjectKey{
				Name:      powervsCluster.Name,
				Namespace: namespace,
			}
			err := testEnv.Get(ctx, key, cluster)
			return err == nil
		}, 10*time.Second).Should(Equal(true))
	}
}

func cleanupCluster(g *WithT, powervsCluster *infrav1.IBMPowerVSCluster, namespace *corev1.Namespace) {
	if powervsCluster != nil {
		func(do ...client.Object) {
			g.Expect(testEnv.Cleanup(ctx, do...)).To(Succeed())
		}(powervsCluster, namespace)
	}
}
