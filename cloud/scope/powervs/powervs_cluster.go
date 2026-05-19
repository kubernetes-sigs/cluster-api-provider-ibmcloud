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
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/go-logr/logr"
	regionUtil "github.com/ppc64le-cloud/powervs-utils"

	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/ibm-cos-sdk-go/aws"
	"github.com/IBM/ibm-cos-sdk-go/aws/awserr"
	cosSession "github.com/IBM/ibm-cos-sdk-go/aws/session"
	"github.com/IBM/ibm-cos-sdk-go/service/s3"
	tgapiv1 "github.com/IBM/networking-go-sdk/transitgatewayapisv1"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/IBM/platform-services-go-sdk/resourcemanagerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/internal/genutil"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/accounts"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/authenticator"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/cos"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcemanager"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/transitgateway"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/endpoints"
)

const (
	// DEBUGLEVEL indicates the debug level of the logs.
	DEBUGLEVEL = 5
)

// vpcSubnetIPVersion4 defines the IP v4 string used for VPC Subnet generation.
var vpcSubnetIPVersion4 = "ipv4"

// networkConnectionType represents network connection type in transit gateway.
type networkConnectionType string

var (
	powervsNetworkConnectionType = networkConnectionType("power_virtual_server")
	vpcNetworkConnectionType     = networkConnectionType("vpc")
)

// powerEdgeRouter is identifier for PER.
const (
	powerEdgeRouter = "power-edge-router"
	// vpcSubnetIPAddressCount is the total IP Addresses for the subnet.
	// Support for custom address prefixes will be added at a later time. Currently, we use the ip count for subnet creation.
	vpcSubnetIPAddressCount int64 = 256
)

// ClusterScopeParams defines the input parameters used to create a new ClusterScope.
type ClusterScopeParams struct {
	Client            client.Client
	Logger            logr.Logger
	Cluster           *clusterv1.Cluster
	IBMPowerVSCluster *infrav1.IBMPowerVSCluster
	ServiceEndpoint   []endpoints.ServiceEndpoint

	// ClientFactory contains collection of functions to override actual client, which helps in testing.
	ClientFactory
}

// ClientFactory is collection of function used for overriding actual clients to help in testing.
type ClientFactory struct {
	AuthenticatorFactory      func() (core.Authenticator, error)
	PowerVSClientFactory      func() (powervs.PowerVS, error)
	VPCClientFactory          func() (vpc.Vpc, error)
	TransitGatewayFactory     func() (transitgateway.TransitGateway, error)
	ResourceControllerFactory func() (resourcecontroller.ResourceController, error)
	ResourceManagerFactory    func() (resourcemanager.ResourceManager, error)
}

// ClusterScope defines a scope defined around a Power VS Cluster.
type ClusterScope struct {
	Client      client.Client
	patchHelper *patch.Helper

	IBMPowerVSClient      powervs.PowerVS
	IBMVPCClient          vpc.Vpc
	TransitGatewayClient  transitgateway.TransitGateway
	ResourceClient        resourcecontroller.ResourceController
	COSClient             cos.Cos
	ResourceManagerClient resourcemanager.ResourceManager

	Cluster           *clusterv1.Cluster
	IBMPowerVSCluster *infrav1.IBMPowerVSCluster
	ServiceEndpoint   []endpoints.ServiceEndpoint
}

func getTGPowerVSConnectionName(tgName string) string { return fmt.Sprintf("%s-pvs-con", tgName) }

func getTGVPCConnectionName(tgName string) string { return fmt.Sprintf("%s-vpc-con", tgName) }

func dhcpNetworkName(dhcpServerName string) string {
	return fmt.Sprintf("DHCPSERVER%s_Private", dhcpServerName)
}

// NewPowerVSClusterScope creates a new ClusterScope from the supplied parameters.
func NewPowerVSClusterScope(params ClusterScopeParams) (*ClusterScope, error) {
	if params.Client == nil {
		err := errors.New("failed to generate new scope as client is nil")
		return nil, err
	}
	if params.Cluster == nil {
		err := errors.New("failed to generate new scope as cluster is nil")
		return nil, err
	}
	if params.IBMPowerVSCluster == nil {
		err := errors.New("failed to generate new scope IBMPowerVSCluster is nil")
		return nil, err
	}
	if params.Logger == (logr.Logger{}) {
		params.Logger = klog.Background()
	}

	helper, err := patch.NewHelper(params.IBMPowerVSCluster, params.Client)
	if err != nil {
		err = fmt.Errorf("failed to init patch helper: %w", err)
		return nil, err
	}

	// If the topology is not explicitly set to LoadBalancer, we only need the PowerVS client
	if params.IBMPowerVSCluster.Spec.Topology != infrav1.PowerVSLoadBalancerTopology {
		return &ClusterScope{
			Client:            params.Client,
			patchHelper:       helper,
			Cluster:           params.Cluster,
			IBMPowerVSCluster: params.IBMPowerVSCluster,
			ServiceEndpoint:   params.ServiceEndpoint,
		}, nil
	}

	// if powervs.cluster.x-k8s.io/create-infra=true annotation is set, create necessary clients.
	piOptions := powervs.ServiceOptions{
		IBMPIOptions: &ibmpisession.IBMPIOptions{
			Debug: params.Logger.V(DEBUGLEVEL).Enabled(),
		},
	}

	// Use Spec.Zone for the PowerVS zone.
	piOptions.Zone = *params.IBMPowerVSCluster.Spec.Zone

	// Get the authenticator.
	auth, err := params.getAuthenticator()
	if err != nil {
		return nil, fmt.Errorf("failed to create authenticator %w", err)
	}
	piOptions.Authenticator = auth

	// Create PowerVS client.
	powerVSClient, err := params.getPowerVSClient(piOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create PowerVS client %w", err)
	}

	// Create VPC client.
	vpcClient, err := params.getVPCClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC client: %w", err)
	}

	// Create TransitGateway client.
	tgOptions := &tgapiv1.TransitGatewayApisV1Options{
		Authenticator: auth,
	}

	tgClient, err := params.getTransitGatewayClient(tgOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create tranist gateway client: %w", err)
	}

	// Create Resource Controller client.
	serviceOption := resourcecontroller.ServiceOptions{
		ResourceControllerV2Options: &resourcecontrollerv2.ResourceControllerV2Options{
			Authenticator: auth,
		},
	}

	resourceClient, err := params.getResourceControllerClient(serviceOption)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource controller client: %w", err)
	}

	// Create Resource Manager client.
	rcManagerOptions := &resourcemanagerv2.ResourceManagerV2Options{
		Authenticator: auth,
	}

	rmClient, err := params.getResourceManagerClient(rcManagerOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource manager client: %w", err)
	}

	clusterScope := &ClusterScope{
		Client:                params.Client,
		patchHelper:           helper,
		Cluster:               params.Cluster,
		IBMPowerVSCluster:     params.IBMPowerVSCluster,
		ServiceEndpoint:       params.ServiceEndpoint,
		IBMPowerVSClient:      powerVSClient,
		IBMVPCClient:          vpcClient,
		TransitGatewayClient:  tgClient,
		ResourceClient:        resourceClient,
		ResourceManagerClient: rmClient,
	}
	return clusterScope, nil
}

func (params ClusterScopeParams) getAuthenticator() (core.Authenticator, error) {
	if params.AuthenticatorFactory != nil {
		return params.AuthenticatorFactory()
	}
	return authenticator.GetAuthenticator()
}

func (params ClusterScopeParams) getPowerVSClient(options powervs.ServiceOptions) (powervs.PowerVS, error) {
	if params.PowerVSClientFactory != nil {
		return params.PowerVSClientFactory()
	}

	// Fetch the PowerVS service endpoint.
	powerVSServiceEndpoint := endpoints.FetchEndpoints(string(endpoints.PowerVS), params.ServiceEndpoint)
	if powerVSServiceEndpoint != "" {
		params.Logger.V(3).Info("Overriding the default PowerVS endpoint", "powerVSEndpoint", powerVSServiceEndpoint)
		options.URL = powerVSServiceEndpoint
	}
	return powervs.NewService(options)
}

func (params ClusterScopeParams) getVPCClient() (vpc.Vpc, error) {
	if params.Logger.V(DEBUGLEVEL).Enabled() {
		core.SetLoggingLevel(core.LevelDebug)
	}
	if params.VPCClientFactory != nil {
		return params.VPCClientFactory()
	}
	if params.IBMPowerVSCluster.Spec.VPC == nil || params.IBMPowerVSCluster.Spec.VPC.Region == nil {
		return nil, fmt.Errorf("failed to create VPC client as VPC info is nil")
	}
	// Fetch the VPC service endpoint.
	svcEndpoint := endpoints.FetchVPCEndpoint(*params.IBMPowerVSCluster.Spec.VPC.Region, params.ServiceEndpoint)
	return vpc.NewService(svcEndpoint)
}

func (params ClusterScopeParams) getTransitGatewayClient(options *tgapiv1.TransitGatewayApisV1Options) (transitgateway.TransitGateway, error) {
	if params.TransitGatewayFactory != nil {
		return params.TransitGatewayFactory()
	}
	// Fetch the TransitGateway service endpoint.
	tgServiceEndpoint := endpoints.FetchEndpoints(string(endpoints.TransitGateway), params.ServiceEndpoint)
	if tgServiceEndpoint != "" {
		params.Logger.V(3).Info("Overriding the default TransitGateway endpoint", "transitGatewayEndpoint", tgServiceEndpoint)
		options.URL = tgServiceEndpoint
	}
	return transitgateway.NewService(options)
}

func (params ClusterScopeParams) getResourceControllerClient(options resourcecontroller.ServiceOptions) (resourcecontroller.ResourceController, error) {
	if params.ResourceControllerFactory != nil {
		return params.ResourceControllerFactory()
	}
	// Fetch the resource controller endpoint.
	rcEndpoint := endpoints.FetchEndpoints(string(endpoints.RC), params.ServiceEndpoint)
	if rcEndpoint != "" {
		options.URL = rcEndpoint
		params.Logger.V(3).Info("Overriding the default resource controller endpoint", "ResourceControllerEndpoint", rcEndpoint)
	}
	return resourcecontroller.NewService(options)
}

func (params ClusterScopeParams) getResourceManagerClient(options *resourcemanagerv2.ResourceManagerV2Options) (resourcemanager.ResourceManager, error) {
	if params.ResourceManagerFactory != nil {
		return params.ResourceManagerFactory()
	}
	// Fetch the resource manager endpoint.
	rmEndpoint := endpoints.FetchEndpoints(string(endpoints.RM), params.ServiceEndpoint)
	if rmEndpoint != "" {
		options.URL = rmEndpoint
		params.Logger.V(3).Info("Overriding the default resource manager endpoint", "ResourceManagerEndpoint", rmEndpoint)
	}
	return resourcemanager.NewService(options)
}

// PatchObject persists the cluster configuration and status.
func (s *ClusterScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.IBMPowerVSCluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *ClusterScope) Close() error {
	return s.PatchObject()
}

// Name returns the CAPI cluster name.
func (s *ClusterScope) Name() string {
	return s.Cluster.Name
}

// Zone returns the cluster zone.
func (s *ClusterScope) Zone() *string {
	return s.IBMPowerVSCluster.Spec.Zone
}

// ResourceGroup returns the cluster resource group.
func (s *ClusterScope) ResourceGroup() *infrav1.IBMPowerVSResourceReference {
	return s.IBMPowerVSCluster.Spec.ResourceGroup
}

// InfraCluster returns the IBMPowerVS infrastructure cluster object name.
func (s *ClusterScope) InfraCluster() string {
	return s.IBMPowerVSCluster.Name
}

// APIServerPort returns the APIServerPort to use when creating the ControlPlaneEndpoint.
func (s *ClusterScope) APIServerPort() int32 {
	if s.Cluster.Spec.ClusterNetwork.APIServerPort > 0 {
		return s.Cluster.Spec.ClusterNetwork.APIServerPort
	}
	return infrav1.DefaultAPIServerPort
}

// SetTransitGatewayConnectionStatus sets the connection status of Transit gateway.
func (s *ClusterScope) SetTransitGatewayConnectionStatus(networkType networkConnectionType, resource *infrav1.ResourceReference) {
	if s.IBMPowerVSCluster.Status.TransitGateway == nil || resource == nil {
		return
	}

	switch networkType {
	case powervsNetworkConnectionType:
		s.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection = resource
	case vpcNetworkConnectionType:
		s.IBMPowerVSCluster.Status.TransitGateway.VPCConnection = resource
	}
}

// SetTransitGatewayStatus sets the status of Transit gateway.
func (s *ClusterScope) SetTransitGatewayStatus(id *string, controllerCreated *bool) {
	s.IBMPowerVSCluster.Status.TransitGateway = &infrav1.TransitGatewayStatus{
		ID:                id,
		ControllerCreated: controllerCreated,
	}
}

// TODO: Can we use generic here.

// SetStatus set the IBMPowerVSCluster status for provided ResourceType.
func (s *ClusterScope) SetStatus(ctx context.Context, resourceType infrav1.ResourceType, resource infrav1.ResourceReference) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Setting status", "resourceType", resourceType, "resource", resource)
	switch resourceType {
	case infrav1.ResourceTypeVPC:
		if s.IBMPowerVSCluster.Status.VPC == nil {
			s.IBMPowerVSCluster.Status.VPC = &resource
			return
		}
		s.IBMPowerVSCluster.Status.VPC.Set(resource)
	case infrav1.ResourceTypeCOSInstance:
		if s.IBMPowerVSCluster.Status.COSInstance == nil {
			s.IBMPowerVSCluster.Status.COSInstance = &resource
			return
		}
		s.IBMPowerVSCluster.Status.COSInstance.Set(resource)
	case infrav1.ResourceTypeResourceGroup:
		if s.IBMPowerVSCluster.Status.ResourceGroup == nil {
			s.IBMPowerVSCluster.Status.ResourceGroup = &resource
			return
		}
		s.IBMPowerVSCluster.Status.ResourceGroup.Set(resource)
	}
}

// VPC returns the cluster VPC information.
func (s *ClusterScope) VPC() *infrav1.VPCResourceReference {
	return s.IBMPowerVSCluster.Spec.VPC
}

// GetVPCID returns the VPC id set in status field of IBMPowerVSCluster object. If it doesn't exist, returns nil.
func (s *ClusterScope) GetVPCID() *string {
	if s.IBMPowerVSCluster.Status.VPC != nil {
		return s.IBMPowerVSCluster.Status.VPC.ID
	}
	return nil
}

// GetVPCSubnetID returns the VPC subnet id.
func (s *ClusterScope) GetVPCSubnetID(subnetName string) *string {
	if s.IBMPowerVSCluster.Status.VPCSubnet == nil {
		return nil
	}
	if val, ok := s.IBMPowerVSCluster.Status.VPCSubnet[subnetName]; ok {
		return val.ID
	}
	return nil
}

// GetVPCSubnetIDs returns all the VPC subnet ids.
func (s *ClusterScope) GetVPCSubnetIDs() []*string {
	subnets := []*string{}
	if s.IBMPowerVSCluster.Status.VPCSubnet == nil {
		return nil
	}
	for _, subnet := range s.IBMPowerVSCluster.Status.VPCSubnet {
		subnets = append(subnets, subnet.ID)
	}
	return subnets
}

// SetVPCSubnetStatus set the VPC subnet id.
func (s *ClusterScope) SetVPCSubnetStatus(ctx context.Context, name string, resource infrav1.ResourceReference) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Setting status", "name", name, "resource", resource)
	if s.IBMPowerVSCluster.Status.VPCSubnet == nil {
		s.IBMPowerVSCluster.Status.VPCSubnet = make(map[string]infrav1.ResourceReference)
	}
	if val, ok := s.IBMPowerVSCluster.Status.VPCSubnet[name]; ok {
		if val.ControllerCreated != nil && *val.ControllerCreated {
			resource.ControllerCreated = val.ControllerCreated
		}
	}
	s.IBMPowerVSCluster.Status.VPCSubnet[name] = resource
}

// GetVPCSecurityGroupByName returns the VPC security group id and its ruleIDs.
func (s *ClusterScope) GetVPCSecurityGroupByName(name string) (*string, []*string, *bool) {
	if s.IBMPowerVSCluster.Status.VPCSecurityGroups == nil {
		return nil, nil, nil
	}
	if val, ok := s.IBMPowerVSCluster.Status.VPCSecurityGroups[name]; ok {
		return val.ID, val.RuleIDs, val.ControllerCreated
	}
	return nil, nil, nil
}

// GetVPCSecurityGroupByID returns the VPC security group's ruleIDs.
func (s *ClusterScope) GetVPCSecurityGroupByID(securityGroupID string) (*string, []*string, *bool) {
	if s.IBMPowerVSCluster.Status.VPCSecurityGroups == nil {
		return nil, nil, nil
	}
	for _, sg := range s.IBMPowerVSCluster.Status.VPCSecurityGroups {
		if *sg.ID == securityGroupID {
			return sg.ID, sg.RuleIDs, sg.ControllerCreated
		}
	}
	return nil, nil, nil
}

// SetVPCSecurityGroupStatus set the VPC security group id.
func (s *ClusterScope) SetVPCSecurityGroupStatus(ctx context.Context, name string, resource infrav1.VPCSecurityGroupStatus) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Setting VPC security group status", "name", name, "resource", resource)
	if s.IBMPowerVSCluster.Status.VPCSecurityGroups == nil {
		s.IBMPowerVSCluster.Status.VPCSecurityGroups = make(map[string]infrav1.VPCSecurityGroupStatus)
	}
	if val, ok := s.IBMPowerVSCluster.Status.VPCSecurityGroups[name]; ok {
		if val.ControllerCreated != nil && *val.ControllerCreated {
			resource.ControllerCreated = val.ControllerCreated
		}
	}
	s.IBMPowerVSCluster.Status.VPCSecurityGroups[name] = resource
}

// TransitGateway returns the cluster Transit Gateway information.
func (s *ClusterScope) TransitGateway() *infrav1.TransitGateway {
	return s.IBMPowerVSCluster.Spec.TransitGateway
}

// GetTransitGatewayID returns the transit gateway id set in status field of IBMPowerVSCluster object. If it doesn't exist, returns empty string.
func (s *ClusterScope) GetTransitGatewayID() *string {
	if s.IBMPowerVSCluster.Status.TransitGateway != nil {
		return s.IBMPowerVSCluster.Status.TransitGateway.ID
	}
	return nil
}

// SetLoadBalancerStatus set the loadBalancer id.
func (s *ClusterScope) SetLoadBalancerStatus(ctx context.Context, name string, loadBalancer infrav1.VPCLoadBalancerStatus) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Setting status", "name", name, "status", loadBalancer)
	if s.IBMPowerVSCluster.Status.LoadBalancers == nil {
		s.IBMPowerVSCluster.Status.LoadBalancers = make(map[string]infrav1.VPCLoadBalancerStatus)
	}
	if val, ok := s.IBMPowerVSCluster.Status.LoadBalancers[name]; ok {
		if val.ControllerCreated != nil && *val.ControllerCreated {
			loadBalancer.ControllerCreated = val.ControllerCreated
		}
	}
	s.IBMPowerVSCluster.Status.LoadBalancers[name] = loadBalancer
}

// GetLoadBalancerID returns the loadBalancer.
func (s *ClusterScope) GetLoadBalancerID(loadBalancerName string) *string {
	if s.IBMPowerVSCluster.Status.LoadBalancers == nil {
		return nil
	}
	if val, ok := s.IBMPowerVSCluster.Status.LoadBalancers[loadBalancerName]; ok {
		return val.ID
	}
	return nil
}

// GetLoadBalancerState will return the state for the load balancer.
func (s *ClusterScope) GetLoadBalancerState(name string) *infrav1.VPCLoadBalancerState {
	if s.IBMPowerVSCluster.Status.LoadBalancers == nil {
		return nil
	}
	if val, ok := s.IBMPowerVSCluster.Status.LoadBalancers[name]; ok {
		return &val.State
	}
	return nil
}

// GetPublicLoadBalancerHostName will return the hostname of the public load balancer.
func (s *ClusterScope) GetPublicLoadBalancerHostName() (*string, error) {
	if s.IBMPowerVSCluster.Status.LoadBalancers == nil {
		return nil, nil
	}

	var name string
	if len(s.IBMPowerVSCluster.Spec.LoadBalancers) == 0 {
		name = *s.GetServiceName(infrav1.ResourceTypeLoadBalancer)
	}

	for _, lb := range s.IBMPowerVSCluster.Spec.LoadBalancers {
		if !*lb.Public {
			continue
		}

		if lb.Name != "" {
			name = lb.Name
			break
		}
		if lb.ID != nil {
			loadBalancer, _, err := s.IBMVPCClient.GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{
				ID: lb.ID,
			})
			if err != nil {
				return nil, err
			}
			name = *loadBalancer.Name
			break
		}
	}

	if val, ok := s.IBMPowerVSCluster.Status.LoadBalancers[name]; ok {
		return val.Hostname, nil
	}
	return nil, nil
}

// GetResourceGroupID returns the resource group id if it present under spec or status filed of IBMPowerVSCluster object
// or returns empty string.
func (s *ClusterScope) GetResourceGroupID() string {
	if s.IBMPowerVSCluster.Spec.ResourceGroup != nil && s.IBMPowerVSCluster.Spec.ResourceGroup.ID != nil {
		return *s.IBMPowerVSCluster.Spec.ResourceGroup.ID
	}
	if s.IBMPowerVSCluster.Status.ResourceGroup != nil && s.IBMPowerVSCluster.Status.ResourceGroup.ID != nil {
		return *s.IBMPowerVSCluster.Status.ResourceGroup.ID
	}
	return ""
}

// IsPowerVSZoneSupportsPER checks whether PowerVS zone supports PER capabilities.
func (s *ClusterScope) IsPowerVSZoneSupportsPER() error {
	zone := s.Zone()
	if zone == nil {
		return fmt.Errorf("PowerVS zone is not set")
	}
	// fetch the datacenter capabilities for zone.
	datacenterCapabilities, err := s.IBMPowerVSClient.GetDatacenterCapabilities(*zone)
	if err != nil {
		return fmt.Errorf("failed to get datacenter capabilities: %w", err)
	}
	// check for the PER support in datacenter capabilities.
	perAvailable, ok := datacenterCapabilities[powerEdgeRouter]
	if !ok {
		return fmt.Errorf("%s capability unknown for zone: %s", powerEdgeRouter, *zone)
	}
	if !perAvailable {
		return fmt.Errorf("%s is not available for zone: %s", powerEdgeRouter, *zone)
	}
	return nil
}

// ReconcileResourceGroup reconciles resource group to fetch resource group id.
func (s *ClusterScope) ReconcileResourceGroup(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)
	// Verify if resource group id is set in spec or status field of IBMPowerVSCluster object.
	if resourceGroupID := s.GetResourceGroupID(); resourceGroupID != "" {
		return nil
	}
	// Try to fetch resource group ID from cloud associated with resource group name.
	resourceGroupID, err := s.fetchResourceGroupID()
	if err != nil {
		return fmt.Errorf("failed to get resource group ID: %w", err)
	}
	log.Info("Fetched resource group ID", "resourceGroupID", resourceGroupID)
	// Set the status of IBMPowerVSCluster object with resource group id.
	s.SetStatus(ctx, infrav1.ResourceTypeResourceGroup, infrav1.ResourceReference{ID: &resourceGroupID, ControllerCreated: ptr.To(false)})
	return nil
}

// ReconcileWorkspace reconciles PowerVS workspace.
func (s *ClusterScope) ReconcileWorkspace(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	cluster := s.IBMPowerVSCluster

	// 1. Idempotency & State Check: If we already resolved the Workspace ID, just verify its state.
	workspaceID := cluster.Status.Workspace.ID
	if workspaceID != "" {
		log.V(3).Info("PowerVS workspace ID is set, fetching details", "workspaceID", workspaceID)

		workspace, _, err := s.ResourceClient.GetResourceInstance(&resourcecontrollerv2.GetResourceInstanceOptions{
			ID: &workspaceID,
		})
		if err != nil {
			return false, fmt.Errorf("failed to fetch workspace (id: %s) details: %w", workspaceID, err)
		}

		if workspace == nil {
			return false, fmt.Errorf("workspace not found with ID: %s", workspaceID)
		}

		requeue, err := s.checkWorkspaceState(ctx, *workspace)
		if err != nil {
			return false, fmt.Errorf("failed to check workspace state: %w", err)
		}
		return requeue, nil
	}

	// 2. We don't have an ID yet. Route logic based strictly on the user's explicit intent.
	log.Info("Resolving PowerVS workspace", "type", cluster.Spec.Workspace.Type)

	switch cluster.Spec.Workspace.Type {
	case infrav1.SourceTypeReference:
		return s.reconcileWorkspaceReference(ctx)

	case infrav1.SourceTypeProvision:
		return s.reconcileWorkspaceProvision(ctx)

	default:
		return false, fmt.Errorf("unknown workspace source type: %s", cluster.Spec.Workspace.Type)
	}
}

// reconcileWorkspaceReference handles the logic when a user brings their own existing workspace.
func (s *ClusterScope) reconcileWorkspaceReference(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	ref := s.IBMPowerVSCluster.Spec.Workspace.Reference

	log.Info("Verifying existing workspace", "reference", ref)

	resourceInstance := resourcecontroller.InstanceFilter{
		ID:             ref.ID,
		Name:           ref.Name,
		Zone:           s.IBMPowerVSCluster.Spec.Zone,
		ResourceID:     resourcecontroller.PowerVSResourceID,
		ResourcePlanID: resourcecontroller.PowerVSResourcePlanID,
	}

	workspace, err := s.ResourceClient.GetResourceInstanceByFilter(resourceInstance)
	if err != nil {
		return false, fmt.Errorf("failed to fetch workspace by ref %q: %w", ref, err)
	}

	if workspace == nil {
		return false, fmt.Errorf("workspace with reference %q not found in IBM Cloud", ref)
	}

	if workspace.GUID == nil || workspace.Name == nil {
		return false, fmt.Errorf("workspace %q has missing GUID or name", ref)
	}

	log.Info("Successfully verified existing workspace", "workspaceID", *workspace.GUID, "name", workspace.Name)

	s.IBMPowerVSCluster.Status.Workspace = infrav1.ResourceReferenceV1Beta3{ID: *workspace.GUID, Name: *workspace.Name}

	return true, nil // requeue so that the state of worksapce will be checked in the next reconcile
}

// reconcileWorkspaceProvision handles the logic when the controller must create a new workspace.
func (s *ClusterScope) reconcileWorkspaceProvision(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	provision := s.IBMPowerVSCluster.Spec.Workspace.Provision

	// 1. Determine the name to use
	workspaceName := provision.Name
	if workspaceName == "" {
		workspaceName = fmt.Sprintf("%s-workspace", s.IBMPowerVSCluster.Name)
	}

	// 2. Idempotency check: Did we already create this, but crash before saving to Status?
	resourceInstance := resourcecontroller.InstanceFilter{
		Name:           workspaceName,
		Zone:           s.IBMPowerVSCluster.Spec.Zone,
		ResourceID:     resourcecontroller.PowerVSResourceID,
		ResourcePlanID: resourcecontroller.PowerVSResourcePlanID,
	}

	workspace, err := s.ResourceClient.GetResourceInstanceByFilter(resourceInstance)
	if err != nil {
		return false, fmt.Errorf("failed to check for existing workspace: %w", err)
	}

	if workspace != nil && workspace.GUID != nil {
		log.Info("Recovered previously provisioned workspace", "workspaceID", workspace.GUID)
		s.IBMPowerVSCluster.Status.Workspace = infrav1.ResourceReferenceV1Beta3{
			ID:   *workspace.GUID,
			Name: workspaceName,
		}
		return true, nil // requeue so that the state of worksapce will be checked in the next reconcile
	}

	// 3. Create the new Workspace
	log.Info("Provisioning new workspace", "name", workspaceName)

	workspace, err = s.createWorkspace(ctx, workspaceName)
	if err != nil {
		return false, fmt.Errorf("failed to provision workspace: %w", err)
	}
	if workspace == nil || workspace.GUID == nil {
		return false, errors.New("provisioned workspace or GUID is nil")
	}

	log.Info("Successfully provisioned workspace", "workspaceID", *workspace.GUID)

	// 4. Save to Status.
	s.IBMPowerVSCluster.Status.Workspace = infrav1.ResourceReferenceV1Beta3{
		ID:   *workspace.GUID,
		Name: workspaceName,
	}

	return true, nil // Requeue to wait for it to become ACTIVE
}

// checkWorkspaceState checks the state of a PowerVS workspace.
// If state is provisioning, true is returned indicating a requeue for reconciliation.
// In all other cases, it returns false.
func (s *ClusterScope) checkWorkspaceState(ctx context.Context, workspace resourcecontrollerv2.ResourceInstance) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Checking the state of PowerVS workspace", "name", *workspace.Name)

	switch *workspace.State {
	case string(infrav1.WorkspaceStateActive):
		log.V(3).Info("PowerVS workspace is in active state")
		return false, nil
	case string(infrav1.WorkspaceStateProvisioning):
		log.V(3).Info("PowerVS workspace is in provisioning state")
		return true, nil
	case string(infrav1.WorkspaceStateFailed):
		return false, fmt.Errorf("PowerVS workspace is in failed state")
	}
	return false, fmt.Errorf("PowerVS workspacee is in %s state", *workspace.State)
}

// createServiceInstance creates the service instance.
func (s *ClusterScope) createWorkspace(ctx context.Context, workspaceName string) (*resourcecontrollerv2.ResourceInstance, error) {
	log := ctrl.LoggerFrom(ctx)

	// fetch resource group id.
	resourceGroupID := s.GetResourceGroupID()
	if resourceGroupID == "" {
		return nil, fmt.Errorf("failed to fetch resource group ID for resource group %v, ID is empty", s.ResourceGroup())
	}

	zone := s.Zone()
	if zone == nil {
		return nil, fmt.Errorf("PowerVS zone is not set")
	}

	// create worksapce.
	log.V(3).Info("Creating new worksapce", "worksapceName", workspaceName, "zone", zone)

	workspace, _, err := s.ResourceClient.CreateResourceInstance(&resourcecontrollerv2.CreateResourceInstanceOptions{
		Name:           &workspaceName,
		Target:         zone,
		ResourceGroup:  &resourceGroupID,
		ResourcePlanID: ptr.To(resourcecontroller.PowerVSResourcePlanID),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create worksapce: %w", err)
	}
	return workspace, nil
}

// ReconcileNetwork reconciles network.
func (s *ClusterScope) ReconcileNetwork(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	cluster := s.IBMPowerVSCluster

	// 1. Idempotency & State Check: If we already resolved the Network ID, just verify its state.
	networkID := cluster.Status.Network.ID
	if networkID != "" {
		log.V(3).Info("PowerVS network ID is set in status, verifying existence", "networkID", networkID)
		if _, err := s.IBMPowerVSClient.GetNetworkByID(networkID); err != nil {
			return false, fmt.Errorf("failed to fetch network by ID: %w", err)
		}

		// If we provisioned this network via DHCP, ensure the DHCP server is fully active
		if cluster.Spec.Network.Type == infrav1.SourceTypeProvision {
			dhcpServerID := cluster.Status.Network.DHCPServer.ID
			if dhcpServerID == "" {
				log.Info("Recovering state: Network ID is present but DHCP Server ID is missing in status. Requeuing to resolve", "networkID", networkID)
				return true, nil
			}

			log.V(3).Info("Verifying provisioned DHCP server state", "dhcpServerID", dhcpServerID)
			active, err := s.isDHCPServerActive(ctx)
			if err != nil {
				return false, fmt.Errorf("failed to check if DHCP server is active: %w", err)
			}

			if !active {
				log.V(3).Info("DHCP server is still building")
				return true, nil // requeue and wait
			}
		}

		// Network is resolved and ready!
		return false, nil
	}

	// 2. We don't have a Network ID yet. Route logic based strictly on the user's explicit intent.
	log.Info("Resolving PowerVS network", "type", cluster.Spec.Network.Type)

	switch cluster.Spec.Network.Type {
	case infrav1.SourceTypeReference:
		return s.reconcileNetworkReference(ctx)

	case infrav1.SourceTypeProvision:
		return s.reconcileNetworkProvision(ctx)

	default:
		return false, fmt.Errorf("unknown network source type: %q", cluster.Spec.Network.Type)
	}
}

// reconcileNetworkReference handles the logic when a user brings their own existing network.
func (s *ClusterScope) reconcileNetworkReference(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	ref := s.IBMPowerVSCluster.Spec.Network.Reference

	log.Info("Verifying existing network", "reference", ref)

	var networkID, networkName string

	if ref.ID != "" {
		network, err := s.IBMPowerVSClient.GetNetworkByID(ref.ID)
		if err != nil {
			return false, fmt.Errorf("failed to fetch network by ID %q: %w", ref.ID, err)
		}
		if network == nil || network.NetworkID == nil || network.Name == nil {
			return false, fmt.Errorf("invalid network payload received from IBM cloud for ID %q: network object, ID, or Name is nil", ref.ID)
		}
		networkID = *network.NetworkID
		networkName = *network.Name
	} else if ref.Name != "" {
		network, err := s.IBMPowerVSClient.GetNetworkByName(ref.Name)
		if err != nil {
			return false, fmt.Errorf("failed to fetch network by name %q: %w", ref.Name, err)
		}
		if network == nil || network.NetworkID == nil || network.Name == nil {
			return false, fmt.Errorf("invalid network payload received from IBM cloud for name %q: network object, ID, or Name is nil", ref.Name)
		}
		networkID = *network.NetworkID
		networkName = *network.Name
	} else {
		return false, fmt.Errorf("network reference must contain either an ID or a Name")
	}

	log.Info("Successfully verified existing network", "networkID", networkID, "networkName", networkName)

	s.IBMPowerVSCluster.Status.Network.ID = networkID
	s.IBMPowerVSCluster.Status.Network.Name = networkName

	return true, nil // requeue so the fast-path verifies it
}

// reconcileNetworkProvision handles the logic when the controller must create a new DHCP server and Network.
func (s *ClusterScope) reconcileNetworkProvision(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	dhcpSpec := s.IBMPowerVSCluster.Spec.Network.Provision.DHCPServer

	// 1. Determine the exact name to use for the DHCP server
	dhcpName := dhcpSpec.Name
	if dhcpName == "" {
		dhcpName = s.IBMPowerVSCluster.Name
	}

	// 2. Idempotency check: Did we already create this DHCP server, but crash before saving to Status?
	dhcpServers, err := s.IBMPowerVSClient.GetAllDHCPServers()
	if err != nil {
		return false, fmt.Errorf("failed to fetch existing DHCP servers for idempotency check: %w", err)
	}

	expectedNetworkName := dhcpNetworkName(dhcpName)
	for _, server := range dhcpServers {
		// Identify by looking at the network name IBM Cloud generated for it
		if server.Network == nil || server.Network.Name == nil || *server.Network.Name != expectedNetworkName {
			continue
		}
		if server.ID == nil || server.Network.ID == nil {
			log.V(4).Info("Skipping malformed DHCP server record from IBM Cloud (missing ID or Network)")
			continue
		}
		log.Info("Recovered previously provisioned DHCP server", "dhcpServerID", *server.ID, "networkID", *server.Network.ID)

		// Save recovered IDs directly to Status
		s.IBMPowerVSCluster.Status.Network.ID = *server.Network.ID
		s.IBMPowerVSCluster.Status.Network.Name = *server.Network.Name
		s.IBMPowerVSCluster.Status.Network.DHCPServer.ID = *server.ID

		return true, nil // requeue
	}

	// 3. Create the new DHCP Server and Network
	log.Info("Provisioning new DHCP Server and Network", "name", dhcpName)

	dhcpServerID, networkID, err := s.createDHCPServer(ctx, dhcpName)
	if err != nil {
		return false, fmt.Errorf("failed to provision DHCP server: %w", err)
	}

	log.Info("Successfully triggered DHCP Server provision", "dhcpServerID", dhcpServerID, "networkID", networkID)

	// 4. Save directly to the new v1beta3 Status value types
	s.IBMPowerVSCluster.Status.Network.ID = networkID
	s.IBMPowerVSCluster.Status.Network.Name = expectedNetworkName
	s.IBMPowerVSCluster.Status.Network.DHCPServer.ID = dhcpServerID

	return true, nil // Requeue to wait for it to become ACTIVE
}

// isDHCPServerActive checks if the DHCP server status is active.
func (s *ClusterScope) isDHCPServerActive(ctx context.Context) (bool, error) {
	dhcpID := s.IBMPowerVSCluster.Status.Network.DHCPServer.ID

	dhcpServer, err := s.IBMPowerVSClient.GetDHCPServer(dhcpID)
	if err != nil {
		return false, err
	}

	if dhcpServer == nil {
		return false, fmt.Errorf("DHCP server details are nil for ID: %s", dhcpID)
	}

	return s.checkDHCPServerStatus(ctx, *dhcpServer)
}

// checkDHCPServerStatus checks the state of a DHCP server.
func (s *ClusterScope) checkDHCPServerStatus(ctx context.Context, dhcpServer models.DHCPServerDetail) (bool, error) {
	log := ctrl.LoggerFrom(ctx)

	if dhcpServer.Status == nil {
		return false, fmt.Errorf("DHCP server status is nil")
	}
	log.V(3).Info("Checking the status of DHCP server", "state", *dhcpServer.Status)

	switch *dhcpServer.Status {
	case string(infrav1.DHCPServerStateActive):
		return true, nil
	case string(infrav1.DHCPServerStateBuild):
		return false, nil
	case string(infrav1.DHCPServerStateError):
		return false, fmt.Errorf("DHCP server creation failed and is in error state")
	default:
		return false, fmt.Errorf("DHCP server is in an unknown state: %s", *dhcpServer.Status)
	}
}

// createDHCPServer creates the DHCP server and returns its ID and its associated Network ID.
func (s *ClusterScope) createDHCPServer(ctx context.Context, dhcpName string) (string, string, error) {
	log := ctrl.LoggerFrom(ctx)

	dhcpSpec := s.IBMPowerVSCluster.Spec.Network.Provision.DHCPServer

	params := models.DHCPServerCreate{
		Name: &dhcpName,
	}

	if dhcpSpec.CIDR != "" {
		params.Cidr = &dhcpSpec.CIDR
	}
	if dhcpSpec.DNSServer != "" {
		params.DNSServer = &dhcpSpec.DNSServer
	}

	snatEnabled := true // default
	if dhcpSpec.Snat == infrav1.DHCPSnatPolicyDisabled {
		snatEnabled = false
	}
	params.SnatEnabled = &snatEnabled

	dhcpServer, err := s.IBMPowerVSClient.CreateDHCPServer(&params)
	if err != nil {
		return "", "", err
	}
	if dhcpServer == nil || dhcpServer.ID == nil {
		return "", "", fmt.Errorf("created DHCP server or its ID is nil")
	}
	if dhcpServer.Network == nil || dhcpServer.Network.ID == nil {
		return "", "", fmt.Errorf("created DHCP server network or its ID is nil")
	}

	log.V(3).Info("DHCP Server network details", "details", *dhcpServer.Network)

	return *dhcpServer.ID, *dhcpServer.Network.ID, nil
}

// ReconcileVPC reconciles VPC.
func (s *ClusterScope) ReconcileVPC(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	// if VPC server id is set means the VPC is already created
	vpcID := s.GetVPCID()
	if vpcID != nil {
		log.V(3).Info("VPC ID is set, fetching details", "vpcID", *vpcID)
		vpcDetails, _, err := s.IBMVPCClient.GetVPC(&vpcv1.GetVPCOptions{
			ID: vpcID,
		})
		if err != nil {
			return false, fmt.Errorf("error fetching VPC details: %w", err)
		}
		if vpcDetails == nil {
			return false, fmt.Errorf("vpc with ID %s not found", *vpcID)
		}

		if vpcDetails.Status != nil && *vpcDetails.Status == string(infrav1.VPCStatePending) {
			log.V(3).Info("VPC creation is in pending state")
			return true, nil
		}
		return false, nil
	}

	log.Info("Checking whether VPC already exist")
	// check vpc exist in cloud
	id, err := s.checkVPC(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check if VPC exists: %w", err)
	}
	if id != "" {
		log.V(3).Info("VPC found in cloud", "vpcID", id)
		s.SetStatus(ctx, infrav1.ResourceTypeVPC, infrav1.ResourceReference{ID: &id, ControllerCreated: ptr.To(false)})
		return false, nil
	}

	// TODO(karthik-k-n): create a generic cluster scope/service and implement common vpc logics, which can be consumed by both vpc and powervs

	// create VPC
	log.Info("Creating a VPC")
	vpcID, err = s.createVPC()
	if err != nil {
		return false, fmt.Errorf("failed to create VPC: %w", err)
	}
	log.Info("Created VPC", "vpcID", *vpcID)
	s.SetStatus(ctx, infrav1.ResourceTypeVPC, infrav1.ResourceReference{ID: vpcID, ControllerCreated: ptr.To(true)})
	return true, nil
}

// checkVPC checks VPC exist in cloud.
func (s *ClusterScope) checkVPC(ctx context.Context) (string, error) {
	var (
		err        error
		vpcDetails *vpcv1.VPC
	)
	log := ctrl.LoggerFrom(ctx)
	if s.IBMPowerVSCluster.Spec.VPC != nil && s.IBMPowerVSCluster.Spec.VPC.ID != nil {
		vpcDetails, _, err = s.IBMVPCClient.GetVPC(&vpcv1.GetVPCOptions{
			ID: s.IBMPowerVSCluster.Spec.VPC.ID,
		})
	} else {
		vpcDetails, err = s.getVPCByName()
	}

	if err != nil {
		return "", fmt.Errorf("failed to get VPC: %w", err)
	}
	if vpcDetails == nil {
		log.Info("VPC not found in cloud", "vpc", s.IBMPowerVSCluster.Spec.VPC)
		return "", nil
	}
	log.Info("VPC found in cloud", "vpcID", *vpcDetails.ID)
	return *vpcDetails.ID, nil
}

func (s *ClusterScope) getVPCByName() (*vpcv1.VPC, error) {
	vpcDetails, err := s.IBMVPCClient.GetVPCByName(*s.GetServiceName(infrav1.ResourceTypeVPC))
	if err != nil {
		return nil, fmt.Errorf("error fetching VPC details with name: %w", err)
	}
	return vpcDetails, nil
}

// createVPC creates VPC.
func (s *ClusterScope) createVPC() (*string, error) {
	resourceGroupID := s.GetResourceGroupID()
	if resourceGroupID == "" {
		return nil, fmt.Errorf("failed to fetch resource group ID for resource group %v, ID is empty", s.ResourceGroup())
	}
	addressPrefixManagement := "auto"
	vpcOption := &vpcv1.CreateVPCOptions{
		ResourceGroup:           &vpcv1.ResourceGroupIdentity{ID: &resourceGroupID},
		Name:                    s.GetServiceName(infrav1.ResourceTypeVPC),
		AddressPrefixManagement: &addressPrefixManagement,
	}
	vpcDetails, _, err := s.IBMVPCClient.CreateVPC(vpcOption)
	if err != nil {
		return nil, err
	}

	// set security group for vpc
	options := &vpcv1.CreateSecurityGroupRuleOptions{}
	options.SetSecurityGroupID(*vpcDetails.DefaultSecurityGroup.ID)
	options.SetSecurityGroupRulePrototype(&vpcv1.SecurityGroupRulePrototype{
		Direction: core.StringPtr("inbound"),
		Protocol:  core.StringPtr("tcp"),
		IPVersion: core.StringPtr("ipv4"),
		PortMin:   core.Int64Ptr(int64(s.APIServerPort())),
		PortMax:   core.Int64Ptr(int64(s.APIServerPort())),
	})
	if _, _, err = s.IBMVPCClient.CreateSecurityGroupRule(options); err != nil {
		return nil, fmt.Errorf("error creating security group rule for VPC: %w", err)
	}
	return vpcDetails.ID, nil
}

// ReconcileVPCSubnets reconciles VPC subnet.
func (s *ClusterScope) ReconcileVPCSubnets(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	subnets := make([]infrav1.Subnet, 0)
	vpcZones, err := regionUtil.VPCZonesForVPCRegion(*s.VPC().Region)
	if err != nil {
		return false, fmt.Errorf("error fetching VPC zones associated with VPC region: %w", err)
	}
	if len(vpcZones) == 0 {
		return false, fmt.Errorf("failed to fetch VPC zones, no zone found for region %s", *s.VPC().Region)
	}
	// check whether user has set the vpc subnets
	if len(s.IBMPowerVSCluster.Spec.VPCSubnets) == 0 {
		// if the user did not set any subnet, we try to create subnet in all the zones.
		log.V(3).Info("VPC subnets details are not set in spec, creating subnets in all zones in the region", "region", *s.VPC().Region)
		for _, zone := range vpcZones {
			subnet := infrav1.Subnet{
				Name: ptr.To(fmt.Sprintf("%s-%s", *s.GetServiceName(infrav1.ResourceTypeSubnet), zone)),
				Zone: ptr.To(zone),
			}
			subnets = append(subnets, subnet)
		}
	} else {
		subnets = append(subnets, s.IBMPowerVSCluster.Spec.VPCSubnets...)
	}

	for index, subnet := range subnets {
		log.Info("Reconciling VPC subnet", "subnet", subnet)
		var subnetID *string
		if subnet.ID != nil {
			subnetID = subnet.ID
		} else {
			if subnet.Name == nil {
				subnet.Name = ptr.To(fmt.Sprintf("%s-%d", *s.GetServiceName(infrav1.ResourceTypeSubnet), index))
			}
			subnetID = s.GetVPCSubnetID(*subnet.Name)
		}

		if subnetID != nil {
			log.V(3).Info("VPC subnet ID is set, fetching details", "subnetID", *subnetID)
			subnetDetails, _, err := s.IBMVPCClient.GetSubnet(&vpcv1.GetSubnetOptions{
				ID: subnetID,
			})
			if err != nil {
				return false, fmt.Errorf("error fetching VPC subnet details: %w", err)
			}
			if subnetDetails == nil {
				return false, fmt.Errorf("failed to get VPC subnet with ID %s", *subnetID)
			}
			// check for next subnet
			s.SetVPCSubnetStatus(ctx, *subnetDetails.Name, infrav1.ResourceReference{ID: subnetDetails.ID})
			continue
		}

		// check VPC subnet exist in cloud
		vpcSubnetID, err := s.checkVPCSubnet(ctx, *subnet.Name)
		if err != nil {
			return false, fmt.Errorf("error checking VPC subnet with name: %w", err)
		}
		if vpcSubnetID != "" {
			log.V(3).Info("Found VPC subnet in cloud", "subnetID", vpcSubnetID)
			s.SetVPCSubnetStatus(ctx, *subnet.Name, infrav1.ResourceReference{ID: &vpcSubnetID, ControllerCreated: ptr.To(false)})
			// check for next subnet
			continue
		}

		if subnet.Zone == nil {
			subnet.Zone = &vpcZones[index%len(vpcZones)]
		}
		log.Info("Creating VPC subnet")
		subnetID, err = s.createVPCSubnet(subnet)
		if err != nil {
			return false, fmt.Errorf("error creating VPC subnet: %w", err)
		}
		log.Info("Created VPC subnet", "subnetID", subnetID)
		s.SetVPCSubnetStatus(ctx, *subnet.Name, infrav1.ResourceReference{ID: subnetID, ControllerCreated: ptr.To(true)})
		// Requeue only when the creation of all subnets has been triggered.
		if index == len(subnets)-1 {
			return true, nil
		}
	}
	return false, nil
}

// checkVPCSubnet checks if VPC subnet by the given name exists in cloud.
func (s *ClusterScope) checkVPCSubnet(ctx context.Context, subnetName string) (string, error) {
	log := ctrl.LoggerFrom(ctx)
	vpcSubnet, err := s.IBMVPCClient.GetVPCSubnetByName(subnetName)
	if err != nil {
		return "", err
	}
	if vpcSubnet == nil {
		log.V(3).Info("VPC subnet not found in cloud", "subnetName", subnetName)
		return "", nil
	}
	return *vpcSubnet.ID, nil
}

// createVPCSubnet creates a VPC subnet.
func (s *ClusterScope) createVPCSubnet(subnet infrav1.Subnet) (*string, error) {
	// TODO(karthik-k-n): consider moving to clusterscope
	// fetch resource group id
	resourceGroupID := s.GetResourceGroupID()
	if resourceGroupID == "" {
		return nil, fmt.Errorf("failed to fetch resource group ID for resource group %v, ID is empty", s.ResourceGroup())
	}

	// create subnet
	vpcID := s.GetVPCID()
	if vpcID == nil {
		return nil, fmt.Errorf("VPC ID is empty")
	}

	ipVersion := vpcSubnetIPVersion4

	options := &vpcv1.CreateSubnetOptions{}
	options.SetSubnetPrototype(&vpcv1.SubnetPrototype{
		IPVersion:             &ipVersion,
		TotalIpv4AddressCount: ptr.To(vpcSubnetIPAddressCount),
		Name:                  subnet.Name,
		VPC: &vpcv1.VPCIdentity{
			ID: vpcID,
		},
		Zone: &vpcv1.ZoneIdentity{
			Name: subnet.Zone,
		},
		ResourceGroup: &vpcv1.ResourceGroupIdentity{
			ID: &resourceGroupID,
		},
	})

	subnetDetails, _, err := s.IBMVPCClient.CreateSubnet(options)
	if err != nil {
		return nil, fmt.Errorf("error creating VPC subnet: %w", err)
	}
	if subnetDetails == nil {
		return nil, fmt.Errorf("created VPC subnet is nil")
	}
	return subnetDetails.ID, nil
}

// ReconcileVPCSecurityGroups reconciles VPC security group.
func (s *ClusterScope) ReconcileVPCSecurityGroups(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)
	for _, securityGroup := range s.IBMPowerVSCluster.Spec.VPCSecurityGroups {
		var securityGroupID *string
		var securityGroupRuleIDs []*string

		if securityGroup.Name != nil {
			securityGroupID, securityGroupRuleIDs, _ = s.GetVPCSecurityGroupByName(*securityGroup.Name)
		} else {
			securityGroupID, securityGroupRuleIDs, _ = s.GetVPCSecurityGroupByID(*securityGroup.ID)
		}

		if securityGroupID != nil && securityGroupRuleIDs != nil {
			if _, _, err := s.IBMVPCClient.GetSecurityGroup(&vpcv1.GetSecurityGroupOptions{
				ID: securityGroupID,
			}); err != nil {
				return fmt.Errorf("failed to fetch existing security group '%s': %w", *securityGroupID, err)
			}
			for _, rule := range securityGroupRuleIDs {
				if _, _, err := s.IBMVPCClient.GetSecurityGroupRule(&vpcv1.GetSecurityGroupRuleOptions{
					SecurityGroupID: securityGroupID,
					ID:              rule,
				}); err != nil {
					return fmt.Errorf("failed to fetch rules of existing security group '%s': %w", *securityGroupID, err)
				}
			}
			continue
		}

		sg, ruleIDs, err := s.validateVPCSecurityGroup(ctx, securityGroup)
		if err != nil {
			return fmt.Errorf("failed to validate existing security group: %w", err)
		}
		if sg != nil {
			log.V(3).Info("VPC security group already exists", "name", *sg.Name)
			s.SetVPCSecurityGroupStatus(ctx, *sg.Name, infrav1.VPCSecurityGroupStatus{
				ID:                sg.ID,
				RuleIDs:           ruleIDs,
				ControllerCreated: ptr.To(false),
			})
			continue
		}

		securityGroupID, err = s.createVPCSecurityGroup(ctx, securityGroup)
		if err != nil {
			return fmt.Errorf("failed to create VPC security group: %w", err)
		}
		log.Info("VPC security group created", "securityGroupName", *securityGroup.Name)
		s.SetVPCSecurityGroupStatus(ctx, *securityGroup.Name, infrav1.VPCSecurityGroupStatus{
			ID:                securityGroupID,
			ControllerCreated: ptr.To(true),
		})

		if err := s.createVPCSecurityGroupRulesAndSetStatus(ctx, securityGroup.Rules, securityGroupID, securityGroup.Name); err != nil {
			return fmt.Errorf("failed to create VPC security group rules: %w", err)
		}
	}

	return nil
}

// createVPCSecurityGroupRule creates a specific rule for a existing security group.
func (s *ClusterScope) createVPCSecurityGroupRule(ctx context.Context, securityGroupID, direction, protocol *string, portMin, portMax *int64, remote infrav1.VPCSecurityGroupRuleRemote) (*string, error) {
	log := ctrl.LoggerFrom(ctx)
	setRemote := func(remote infrav1.VPCSecurityGroupRuleRemote, remoteOption *vpcv1.SecurityGroupRuleRemotePrototype) error {
		switch remote.RemoteType {
		case infrav1.VPCSecurityGroupRuleRemoteTypeCIDR:
			cidrSubnet, err := s.IBMVPCClient.GetVPCSubnetByName(*remote.CIDRSubnetName)
			if err != nil {
				return fmt.Errorf("failed to find VPC subnet by name '%s' for fetching CIDR block: %w", *remote.CIDRSubnetName, err)
			}
			if cidrSubnet == nil {
				return fmt.Errorf("VPC subnet by name '%s' does not exist", *remote.CIDRSubnetName)
			}
			log.V(3).Info("Creating VPC security group rule", "securityGroupID", *securityGroupID, "direction", *direction, "protocol", *protocol, "cidrBlockSubnet", *remote.CIDRSubnetName, "cidr", *cidrSubnet.Ipv4CIDRBlock)
			remoteOption.CIDRBlock = cidrSubnet.Ipv4CIDRBlock
		case infrav1.VPCSecurityGroupRuleRemoteTypeAddress:
			log.V(3).Info("Creating VPC security group rule", "securityGroupID", *securityGroupID, "direction", *direction, "protocol", *protocol, "ip", *remote.Address)
			remoteOption.Address = remote.Address
		case infrav1.VPCSecurityGroupRuleRemoteTypeSG:
			sg, err := s.IBMVPCClient.GetSecurityGroupByName(*remote.SecurityGroupName)
			if err != nil {
				return fmt.Errorf("failed to find VPC security group by name '%s', err: %w", *remote.SecurityGroupName, err)
			}
			if sg == nil {
				return fmt.Errorf("VPC security group by name '%s' does not exist", *remote.SecurityGroupName)
			}
			log.V(3).Info("Creating VPC security group rule", "securityGroupID", *securityGroupID, "direction", *direction, "protocol", *protocol, "securityGroup", *remote.SecurityGroupName, "securityGroupCRN", *sg.CRN)
			remoteOption.CRN = sg.CRN
		default:
			log.V(3).Info("Creating VPC security group rule", "securityGroupID", *securityGroupID, "direction", *direction, "protocol", *protocol, "cidr", "0.0.0.0/0")
			remoteOption.CIDRBlock = ptr.To("0.0.0.0/0")
		}

		return nil
	}

	remoteOption := &vpcv1.SecurityGroupRuleRemotePrototype{}
	if err := setRemote(remote, remoteOption); err != nil {
		return nil, fmt.Errorf("failed to set remote option while creating VPC security group rule: %w", err)
	}

	options := vpcv1.CreateSecurityGroupRuleOptions{
		SecurityGroupID: securityGroupID,
	}

	options.SetSecurityGroupRulePrototype(&vpcv1.SecurityGroupRulePrototype{
		Direction: direction,
		Protocol:  protocol,
		PortMin:   portMin,
		PortMax:   portMax,
		Remote:    remoteOption,
	})

	var ruleID *string
	ruleIntf, _, err := s.IBMVPCClient.CreateSecurityGroupRule(&options)
	if err != nil {
		return nil, fmt.Errorf("failed to create VPC security group rule: %w", err)
	}

	switch reflect.TypeOf(ruleIntf).String() {
	case "*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll":
		rule := ruleIntf.(*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll)
		ruleID = rule.ID
	case "*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp":
		rule := ruleIntf.(*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp)
		ruleID = rule.ID
	case "*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolIcmp":
		rule := ruleIntf.(*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolIcmp)
		ruleID = rule.ID
	}
	log.Info("Created VPC security group rule", "ruleID", *ruleID)
	return ruleID, nil
}

// createVPCSecurityGroupRules creates rules for a security group.
func (s *ClusterScope) createVPCSecurityGroupRules(ctx context.Context, ogSecurityGroupRules []*infrav1.VPCSecurityGroupRule, securityGroupID *string) ([]*string, error) {
	log := ctrl.LoggerFrom(ctx)
	var ruleIDs []*string
	log.V(3).Info("Creating VPC security group rules")

	for _, rule := range ogSecurityGroupRules {
		var protocol *string
		var portMax, portMin *int64

		direction := ptr.To(string(rule.Direction))
		switch rule.Direction {
		case infrav1.VPCSecurityGroupRuleDirectionInbound:
			protocol = ptr.To(string(rule.Source.Protocol))
			if rule.Source.PortRange != nil {
				portMin = ptr.To(rule.Source.PortRange.MinimumPort)
				portMax = ptr.To(rule.Source.PortRange.MaximumPort)
			}

			for _, remote := range rule.Source.Remotes {
				id, err := s.createVPCSecurityGroupRule(ctx, securityGroupID, direction, protocol, portMin, portMax, remote)
				if err != nil {
					return nil, fmt.Errorf("failed to create VPC security group rule: %w", err)
				}
				ruleIDs = append(ruleIDs, id)
			}
		case infrav1.VPCSecurityGroupRuleDirectionOutbound:
			protocol = ptr.To(string(rule.Destination.Protocol))
			if rule.Destination.PortRange != nil {
				portMin = ptr.To(rule.Destination.PortRange.MinimumPort)
				portMax = ptr.To(rule.Destination.PortRange.MaximumPort)
			}

			for _, remote := range rule.Destination.Remotes {
				id, err := s.createVPCSecurityGroupRule(ctx, securityGroupID, direction, protocol, portMin, portMax, remote)
				if err != nil {
					return nil, fmt.Errorf("failed to create VPC security group rule: %w", err)
				}
				ruleIDs = append(ruleIDs, id)
			}
		}
	}

	return ruleIDs, nil
}

// createVPCSecurityGroupRulesAndSetStatus creates VPC security group rules and sets its status.
func (s *ClusterScope) createVPCSecurityGroupRulesAndSetStatus(ctx context.Context, ogSecurityGroupRules []*infrav1.VPCSecurityGroupRule, securityGroupID, securityGroupName *string) error {
	log := ctrl.LoggerFrom(ctx)
	ruleIDs, err := s.createVPCSecurityGroupRules(ctx, ogSecurityGroupRules, securityGroupID)
	if err != nil {
		return fmt.Errorf("failed to create VPC security group rules: %w", err)
	}
	log.Info("VPC security group rules created", "securityGroupName", *securityGroupName)

	s.SetVPCSecurityGroupStatus(ctx, *securityGroupName, infrav1.VPCSecurityGroupStatus{
		ID:                securityGroupID,
		RuleIDs:           ruleIDs,
		ControllerCreated: ptr.To(true),
	})

	return nil
}

// createVPCSecurityGroup creates a VPC security group.
func (s *ClusterScope) createVPCSecurityGroup(ctx context.Context, spec infrav1.VPCSecurityGroup) (*string, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Creating VPC security group", "name", *spec.Name)

	options := &vpcv1.CreateSecurityGroupOptions{
		VPC: &vpcv1.VPCIdentity{
			ID: s.GetVPCID(),
		},
		Name: spec.Name,
		ResourceGroup: &vpcv1.ResourceGroupIdentity{
			ID: ptr.To(s.GetResourceGroupID()),
		},
	}

	securityGroup, _, err := s.IBMVPCClient.CreateSecurityGroup(options)
	if err != nil {
		return nil, fmt.Errorf("error creating VPC security group: %w", err)
	}
	// To-Do: Add tags to VPC security group, need to implement the client for "github.com/IBM/platform-services-go-sdk/globaltaggingv1".
	return securityGroup.ID, nil
}

// validateVPCSecurityGroupRuleRemote compares a specific security group rule's remote with the spec and existing security group rule's remote.
func (s *ClusterScope) validateVPCSecurityGroupRuleRemote(originalSGRemote *vpcv1.SecurityGroupRuleRemote, expectedSGRemote infrav1.VPCSecurityGroupRuleRemote) (bool, error) {
	var match bool

	switch expectedSGRemote.RemoteType {
	case infrav1.VPCSecurityGroupRuleRemoteTypeAny:
		if originalSGRemote.CIDRBlock != nil && *originalSGRemote.CIDRBlock == "0.0.0.0/0" {
			match = true
		}
	case infrav1.VPCSecurityGroupRuleRemoteTypeAddress:
		if originalSGRemote.Address != nil && *originalSGRemote.Address == *expectedSGRemote.Address {
			match = true
		}
	case infrav1.VPCSecurityGroupRuleRemoteTypeCIDR:
		cidrSubnet, err := s.IBMVPCClient.GetVPCSubnetByName(*expectedSGRemote.CIDRSubnetName)
		if err != nil {
			return false, fmt.Errorf("failed to find VPC subnet by name '%s' for fetching CIDR block: %w", *expectedSGRemote.CIDRSubnetName, err)
		}

		if originalSGRemote.CIDRBlock != nil && cidrSubnet != nil && *originalSGRemote.CIDRBlock == *cidrSubnet.Ipv4CIDRBlock {
			match = true
		}
	case infrav1.VPCSecurityGroupRuleRemoteTypeSG:
		securityGroup, err := s.IBMVPCClient.GetSecurityGroupByName(*expectedSGRemote.SecurityGroupName)
		if err != nil {
			return false, fmt.Errorf("failed to find ID for resource group '%s': %w", *expectedSGRemote.SecurityGroupName, err)
		}

		if originalSGRemote.CRN != nil && securityGroup.Name != nil && *originalSGRemote.CRN == *securityGroup.CRN {
			match = true
		}
	}

	return match, nil
}

// validateSecurityGroupRule compares a specific security group's rule with the spec and existing security group's rule.
func (s *ClusterScope) validateSecurityGroupRule(originalSecurityGroupRules []vpcv1.SecurityGroupRuleIntf, direction infrav1.VPCSecurityGroupRuleDirection, rule *infrav1.VPCSecurityGroupRulePrototype, remote infrav1.VPCSecurityGroupRuleRemote) (ruleID *string, match bool, err error) {
	updateError := func(e error) {
		err = fmt.Errorf("failed to validate VPC security group rule's remote: %w", e)
	}

	protocol := string(rule.Protocol)

	for _, ogRuleIntf := range originalSecurityGroupRules {
		switch reflect.TypeOf(ogRuleIntf).String() {
		case "*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll":
			ogRule := ogRuleIntf.(*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolAll)
			ruleID = ogRule.ID

			if *ogRule.Direction == string(direction) && *ogRule.Protocol == protocol {
				ogRemote := ogRule.Remote.(*vpcv1.SecurityGroupRuleRemote)
				match, err = s.validateVPCSecurityGroupRuleRemote(ogRemote, remote)
				if err != nil {
					updateError(err)
					return nil, false, err
				}
			}
		case "*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp":
			portMin := rule.PortRange.MinimumPort
			portMax := rule.PortRange.MaximumPort
			ogRule := ogRuleIntf.(*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp)
			ruleID = ogRule.ID

			if *ogRule.Direction == string(direction) && *ogRule.Protocol == protocol && *ogRule.PortMax == portMax && *ogRule.PortMin == portMin {
				ogRemote := ogRule.Remote.(*vpcv1.SecurityGroupRuleRemote)
				match, err = s.validateVPCSecurityGroupRuleRemote(ogRemote, remote)
				if err != nil {
					updateError(err)
					return nil, false, err
				}
			}
		case "*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolIcmp":
			icmpCode := rule.ICMPCode
			icmpType := rule.ICMPType
			ogRule := ogRuleIntf.(*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolIcmp)
			ruleID = ogRule.ID

			if *ogRule.Direction == string(direction) && *ogRule.Protocol == protocol && *ogRule.Code == *icmpCode && *ogRule.Type == *icmpType {
				ogRemote := ogRule.Remote.(*vpcv1.SecurityGroupRuleRemote)
				match, err = s.validateVPCSecurityGroupRuleRemote(ogRemote, remote)
				if err != nil {
					updateError(err)
					return nil, false, err
				}
			}
		}
		if match {
			return ruleID, match, nil
		}
	}

	return nil, false, nil
}

// validateVPCSecurityGroupRules compares a specific security group rules spec with the existing security group's rules.
func (s *ClusterScope) validateVPCSecurityGroupRules(originalSecurityGroupRules []vpcv1.SecurityGroupRuleIntf, expectedSecurityGroupRules []*infrav1.VPCSecurityGroupRule) ([]*string, bool, error) {
	ruleIDs := []*string{}
	for _, expectedRule := range expectedSecurityGroupRules {
		direction := expectedRule.Direction

		switch direction {
		case infrav1.VPCSecurityGroupRuleDirectionInbound:
			for _, remote := range expectedRule.Source.Remotes {
				id, match, err := s.validateSecurityGroupRule(originalSecurityGroupRules, direction, expectedRule.Source, remote)
				if err != nil {
					return nil, false, fmt.Errorf("failed to validate VPC security group rule: %w", err)
				}
				if !match {
					return nil, false, nil
				}
				ruleIDs = append(ruleIDs, id)
			}
		case infrav1.VPCSecurityGroupRuleDirectionOutbound:
			for _, remote := range expectedRule.Destination.Remotes {
				id, match, err := s.validateSecurityGroupRule(originalSecurityGroupRules, direction, expectedRule.Destination, remote)
				if err != nil {
					return nil, false, fmt.Errorf("failed to validate VPC security group rule: %v", err)
				}
				if !match {
					return nil, false, nil
				}
				ruleIDs = append(ruleIDs, id)
			}
		}
	}

	return ruleIDs, true, nil
}

// validateVPCSecurityGroup validates the security group and it's rules provided by user via spec.
func (s *ClusterScope) validateVPCSecurityGroup(ctx context.Context, securityGroup infrav1.VPCSecurityGroup) (*vpcv1.SecurityGroup, []*string, error) {
	var securityGroupDet *vpcv1.SecurityGroup
	var err error

	if securityGroup.ID != nil {
		securityGroupDet, _, err = s.IBMVPCClient.GetSecurityGroup(&vpcv1.GetSecurityGroupOptions{
			ID: securityGroup.ID,
		})
		if err != nil {
			return nil, nil, err
		}
		if securityGroupDet == nil {
			return nil, nil, fmt.Errorf("failed to find VPC security group with provided ID '%v'", securityGroup.ID)
		}
	} else {
		securityGroupDet, err = s.IBMVPCClient.GetSecurityGroupByName(*securityGroup.Name)
		if err != nil {
			if _, ok := err.(*vpc.SecurityGroupByNameNotFound); !ok {
				return nil, nil, err
			}
		}
		if securityGroupDet == nil {
			return nil, nil, nil
		}
	}
	if securityGroupDet.VPC == nil || securityGroupDet.VPC.ID == nil || *securityGroupDet.VPC.ID != *s.GetVPCID() {
		return nil, nil, fmt.Errorf("VPC security group by name exists but is not attached to VPC")
	}

	ruleIDs, ok, err := s.validateVPCSecurityGroupRules(securityGroupDet.Rules, securityGroup.Rules)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to validate VPC security group rules: %v", err)
	}
	if !ok {
		if _, _, controllerCreated := s.GetVPCSecurityGroupByName(*securityGroup.Name); controllerCreated != nil && !*controllerCreated {
			return nil, nil, fmt.Errorf("VPC security group by name exists but rules are not matching")
		}
		return nil, nil, s.createVPCSecurityGroupRulesAndSetStatus(ctx, securityGroup.Rules, securityGroupDet.ID, securityGroupDet.Name)
	}

	return securityGroupDet, ruleIDs, nil
}

// ReconcileTransitGateway reconcile transit gateway.
func (s *ClusterScope) ReconcileTransitGateway(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	if s.GetTransitGatewayID() != nil {
		log.V(3).Info("Transit gateway ID is set, fetching details", "transitGatewayID", s.GetTransitGatewayID())
		tg, _, err := s.TransitGatewayClient.GetTransitGateway(&tgapiv1.GetTransitGatewayOptions{
			ID: s.GetTransitGatewayID(),
		})
		if err != nil {
			return false, fmt.Errorf("failed to get transit gateway: %w", err)
		}
		requeue, err := s.checkAndUpdateTransitGateway(ctx, tg)
		if err != nil {
			return false, fmt.Errorf("failed to check and update transit gateway: %w", err)
		}
		return requeue, nil
	}

	// check transit gateway exist in cloud
	tg, err := s.isTransitGatewayExists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check if transit gateway exists: %w", err)
	}

	// check the status and update the transit gateway's connections if they are not proper
	if tg != nil {
		requeue, err := s.checkAndUpdateTransitGateway(ctx, tg)
		if err != nil {
			return false, fmt.Errorf("failed to check and update transit gateway connections: %w", err)
		}
		return requeue, nil
	}

	// create transit gateway
	log.Info("Creating transit gateway")
	if err := s.createTransitGateway(ctx); err != nil {
		return false, fmt.Errorf("failed to create transit gateway: %w", err)
	}

	return true, nil
}

// isTransitGatewayExists checks transit gateway exist in cloud.
func (s *ClusterScope) isTransitGatewayExists(ctx context.Context) (*tgapiv1.TransitGateway, error) {
	log := ctrl.LoggerFrom(ctx)
	// TODO(karthik-k-n): Support regex
	var transitGateway *tgapiv1.TransitGateway
	var err error

	if s.IBMPowerVSCluster.Spec.TransitGateway != nil && s.IBMPowerVSCluster.Spec.TransitGateway.ID != nil {
		transitGateway, _, err = s.TransitGatewayClient.GetTransitGateway(&tgapiv1.GetTransitGatewayOptions{
			ID: s.IBMPowerVSCluster.Spec.TransitGateway.ID,
		})
	} else {
		transitGateway, err = s.TransitGatewayClient.GetTransitGatewayByName(*s.GetServiceName(infrav1.ResourceTypeTransitGateway))
	}

	if err != nil {
		return nil, err
	}

	if transitGateway == nil || transitGateway.ID == nil {
		log.Info("Transit gateway not found in cloud")
		return nil, nil
	}

	s.SetTransitGatewayStatus(transitGateway.ID, ptr.To(false))

	return transitGateway, nil
}

// checkAndUpdateTransitGateway checks given transit gateway's status and its connections.
// if update is set to true, it updates the transit gateway connections too if it is not exist already.
func (s *ClusterScope) checkAndUpdateTransitGateway(ctx context.Context, transitGateway *tgapiv1.TransitGateway) (bool, error) {
	requeue, err := s.checkTransitGatewayStatus(ctx, transitGateway)
	if err != nil {
		return false, err
	}
	if requeue {
		return requeue, nil
	}

	return s.checkAndUpdateTransitGatewayConnections(ctx, transitGateway)
}

// checkTransitGatewayStatus checks the state of a transit gateway.
// If state is pending, true is returned indicating a requeue for reconciliation.
// In all other cases, it returns false.
func (s *ClusterScope) checkTransitGatewayStatus(ctx context.Context, tg *tgapiv1.TransitGateway) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Checking the status of transit gateway", "name", *tg.Name)
	switch *tg.Status {
	case string(infrav1.TransitGatewayStateAvailable):
		log.V(3).Info("Transit gateway is in available state")
	case string(infrav1.TransitGatewayStateFailed):
		return false, fmt.Errorf("failed to create transit gateway, current status: %s", *tg.Status)
	case string(infrav1.TransitGatewayStatePending):
		log.V(3).Info("Transit gateway is in pending state")
		return true, nil
	}

	return false, nil
}

// checkAndUpdateTransitGatewayConnections checks given transit gateway's connections status.
// it also creates the transit gateway connections if it is not exist already.
func (s *ClusterScope) checkAndUpdateTransitGatewayConnections(ctx context.Context, transitGateway *tgapiv1.TransitGateway) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	tgConnections, _, err := s.TransitGatewayClient.ListTransitGatewayConnections(&tgapiv1.ListTransitGatewayConnectionsOptions{
		TransitGatewayID: transitGateway.ID,
	})
	if err != nil {
		return false, fmt.Errorf("failed to list transit gateway connections: %w", err)
	}

	vpcCRN, err := s.fetchVPCCRN()
	if err != nil {
		return false, fmt.Errorf("failed to fetch VPC CRN: %w", err)
	}

	pvsServiceInstanceCRN, err := s.fetchPowerVSServiceInstanceCRN()
	if err != nil {
		return false, fmt.Errorf("failed to fetch PowerVS service instance CRN: %w", err)
	}

	if len(tgConnections.Connections) == 0 {
		log.Info("Connections not exist on transit gateway, creating them")
		if err := s.createTransitGatewayConnections(ctx, transitGateway, pvsServiceInstanceCRN, vpcCRN); err != nil {
			return false, fmt.Errorf("failed to create transit gateway connections: %w", err)
		}

		return true, nil
	}

	requeue, powerVSConnStatus, vpcConnStatus, err := s.validateTransitGatewayConnections(ctx, tgConnections.Connections, vpcCRN, pvsServiceInstanceCRN)
	if err != nil {
		return false, fmt.Errorf("failed to validate transit gateway connections: %w", err)
	} else if requeue {
		return requeue, nil
	}

	// return when connections are in attached state.
	if powerVSConnStatus && vpcConnStatus {
		return false, nil
	}

	// update the connections when connection not exist
	if !powerVSConnStatus {
		log.V(3).Info("Only PowerVS connection not exist in transit gateway, creating it")
		if err := s.createTransitGatewayConnection(ctx, transitGateway.ID, ptr.To(getTGPowerVSConnectionName(*transitGateway.Name)), pvsServiceInstanceCRN, powervsNetworkConnectionType); err != nil {
			return false, fmt.Errorf("failed to create PowerVS transit gateway connection: %w", err)
		}
	}

	if !vpcConnStatus {
		log.V(3).Info("Only VPC connection not exist in transit gateway, creating it")
		if err := s.createTransitGatewayConnection(ctx, transitGateway.ID, ptr.To(getTGVPCConnectionName(*transitGateway.Name)), vpcCRN, vpcNetworkConnectionType); err != nil {
			return false, fmt.Errorf("failed to create VPC transit gateway connection: %w", err)
		}
	}

	return true, nil
}

// validateTransitGatewayConnections validates the existing transit gateway connections.
// to avoid returning many return values, connection ID will be returned and considered that connection is in attached state.
func (s *ClusterScope) validateTransitGatewayConnections(ctx context.Context, connections []tgapiv1.TransitGatewayConnectionCust, vpcCRN, pvsServiceInstanceCRN *string) (bool, bool, bool, error) {
	var powerVSConnStatus, vpcConnStatus bool
	for _, conn := range connections {
		if *conn.NetworkType == string(vpcNetworkConnectionType) && *conn.NetworkID == *vpcCRN {
			if requeue, err := s.checkTransitGatewayConnectionStatus(ctx, conn); err != nil {
				return requeue, false, false, fmt.Errorf("failed to check VPC transit gateway connection status: %w", err)
			} else if requeue {
				return requeue, false, false, nil
			}

			if s.IBMPowerVSCluster.Status.TransitGateway != nil && s.IBMPowerVSCluster.Status.TransitGateway.VPCConnection == nil {
				s.SetTransitGatewayConnectionStatus(vpcNetworkConnectionType, &infrav1.ResourceReference{ID: conn.ID, ControllerCreated: ptr.To(false)})
			}
			vpcConnStatus = true
		}
		if *conn.NetworkType == string(powervsNetworkConnectionType) && *conn.NetworkID == *pvsServiceInstanceCRN {
			if requeue, err := s.checkTransitGatewayConnectionStatus(ctx, conn); err != nil {
				return requeue, false, false, fmt.Errorf("failed to check PowerVS transit gateway connection status: %w", err)
			} else if requeue {
				return requeue, false, false, nil
			}

			if s.IBMPowerVSCluster.Status.TransitGateway != nil && s.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection == nil {
				s.SetTransitGatewayConnectionStatus(powervsNetworkConnectionType, &infrav1.ResourceReference{ID: conn.ID, ControllerCreated: ptr.To(false)})
			}
			powerVSConnStatus = true
		}
	}

	return false, powerVSConnStatus, vpcConnStatus, nil
}

// checkTransitGatewayConnectionStatus checks the state of a transit gateway connection.
// If state is pending, true is returned indicating a requeue for reconciliation.
// In all other cases, it returns false.
func (s *ClusterScope) checkTransitGatewayConnectionStatus(ctx context.Context, con tgapiv1.TransitGatewayConnectionCust) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Checking the status of transit gateway connection", "name", *con.Name)
	switch *con.Status {
	case string(infrav1.TransitGatewayConnectionStateAttached):
		return false, nil
	case string(infrav1.TransitGatewayConnectionStateFailed):
		return false, fmt.Errorf("failed to attach connection to transit gateway, current status: %s", *con.Status)
	case string(infrav1.TransitGatewayConnectionStatePending):
		log.V(3).Info("Transit gateway connection is in pending state")
		return true, nil
	}
	return false, nil
}

// createTransitGatewayConnection creates transit gateway connection and sets the connection status.
func (s *ClusterScope) createTransitGatewayConnection(ctx context.Context, transitGatewayID, connName, networkID *string, networkType networkConnectionType) error {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Creating transit gateway connection", "transitGatewayID", transitGatewayID, "connectionType", networkType, "connectionName", connName)
	conn, _, err := s.TransitGatewayClient.CreateTransitGatewayConnection(&tgapiv1.CreateTransitGatewayConnectionOptions{
		TransitGatewayID: transitGatewayID,
		NetworkType:      ptr.To(string(networkType)),
		NetworkID:        networkID,
		Name:             connName,
	})
	if err != nil {
		return err
	}
	s.SetTransitGatewayConnectionStatus(networkType, &infrav1.ResourceReference{ID: conn.ID, ControllerCreated: ptr.To(true)})

	return nil
}

// createTransitGatewayConnections creates PowerVS and VPC connections in the transit gateway.
func (s *ClusterScope) createTransitGatewayConnections(ctx context.Context, tg *tgapiv1.TransitGateway, pvsServiceInstanceCRN, vpcCRN *string) error {
	if err := s.createTransitGatewayConnection(ctx, tg.ID, ptr.To(getTGPowerVSConnectionName(*tg.Name)), pvsServiceInstanceCRN, powervsNetworkConnectionType); err != nil {
		return fmt.Errorf("failed to create PowerVS connection in transit gateway: %w", err)
	}

	if err := s.createTransitGatewayConnection(ctx, tg.ID, ptr.To(getTGVPCConnectionName(*tg.Name)), vpcCRN, vpcNetworkConnectionType); err != nil {
		return fmt.Errorf("failed to create VPC connection in transit gateway: %w", err)
	}

	return nil
}

// createTransitGateway creates transit gateway and sets the transit gateway status.
func (s *ClusterScope) createTransitGateway(ctx context.Context) error {
	// fetch resource group id
	resourceGroupID := s.GetResourceGroupID()
	if resourceGroupID == "" {
		return fmt.Errorf("failed to fetch resource group ID for resource group %v, ID is empty", s.ResourceGroup())
	}

	if s.IBMPowerVSCluster.Status.Workspace.ID == "" || s.IBMPowerVSCluster.Status.VPC == nil {
		return fmt.Errorf("failed to proceed with transit gateway creation: PowerVS workspace or VPC reconciliation is not yet complete")
	}

	location, globalRouting, err := genutil.GetTransitGatewayLocationAndRouting(s.Zone(), s.VPC().Region)
	if err != nil {
		return fmt.Errorf("failed to get transit gateway location and routing: %w", err)
	}

	// throw error when user tries to use local routing where global routing is required.
	// TODO: Add a webhook validation for below condition.
	if s.IBMPowerVSCluster.Spec.TransitGateway != nil && s.IBMPowerVSCluster.Spec.TransitGateway.GlobalRouting != nil && !*s.IBMPowerVSCluster.Spec.TransitGateway.GlobalRouting && *globalRouting {
		return fmt.Errorf("failed to use local routing for transit gateway since PowerVS and VPC are in different region and requires global routing")
	}
	// setting global routing to true when it is set by user.
	if s.IBMPowerVSCluster.Spec.TransitGateway != nil && s.IBMPowerVSCluster.Spec.TransitGateway.GlobalRouting != nil && *s.IBMPowerVSCluster.Spec.TransitGateway.GlobalRouting {
		globalRouting = ptr.To(true)
	}

	tgName := s.GetServiceName(infrav1.ResourceTypeTransitGateway)
	tg, _, err := s.TransitGatewayClient.CreateTransitGateway(&tgapiv1.CreateTransitGatewayOptions{
		Location:      location,
		Name:          tgName,
		Global:        globalRouting,
		ResourceGroup: &tgapiv1.ResourceGroupIdentity{ID: ptr.To(resourceGroupID)},
	})
	if err != nil {
		return err
	}

	s.SetTransitGatewayStatus(tg.ID, ptr.To(true))

	vpcCRN, err := s.fetchVPCCRN()
	if err != nil {
		return fmt.Errorf("failed to fetch VPC CRN: %w", err)
	}
	pvsServiceInstanceCRN, err := s.fetchPowerVSServiceInstanceCRN()
	if err != nil {
		return fmt.Errorf("failed to fetch PowerVS service instance CRN: %w", err)
	}

	if err := s.createTransitGatewayConnections(ctx, tg, pvsServiceInstanceCRN, vpcCRN); err != nil {
		return fmt.Errorf("failed to create connection in transit gateway: %w", err)
	}

	return nil
}

// ReconcileLoadBalancers reconcile loadBalancer.
func (s *ClusterScope) ReconcileLoadBalancers(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	loadBalancers := make([]infrav1.VPCLoadBalancerSpec, 0)
	if len(s.IBMPowerVSCluster.Spec.LoadBalancers) == 0 {
		log.V(3).Info("VPC load balancer is not set, constructing one")
		loadBalancer := infrav1.VPCLoadBalancerSpec{
			Name:   *s.GetServiceName(infrav1.ResourceTypeLoadBalancer),
			Public: ptr.To(true),
		}
		loadBalancers = append(loadBalancers, loadBalancer)
	} else {
		loadBalancers = append(loadBalancers, s.IBMPowerVSCluster.Spec.LoadBalancers...)
	}

	isAnyLoadBalancerNotReady := false

	for index, loadBalancer := range loadBalancers {
		var loadBalancerID *string
		if loadBalancer.ID != nil {
			loadBalancerID = loadBalancer.ID
		} else {
			if loadBalancer.Name == "" {
				loadBalancer.Name = fmt.Sprintf("%s-%d", *s.GetServiceName(infrav1.ResourceTypeLoadBalancer), index)
			}
			loadBalancerID = s.GetLoadBalancerID(loadBalancer.Name)
		}
		if loadBalancerID != nil {
			log.V(3).Info("Load balancer ID is set, fetching load balancer details", "loadBalancerID", *loadBalancerID)
			loadBalancer, _, err := s.IBMVPCClient.GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{
				ID: loadBalancerID,
			})
			if err != nil {
				return false, fmt.Errorf("failed to fetch load balancer details: %w", err)
			}

			if isReady := s.checkLoadBalancerStatus(ctx, *loadBalancer); !isReady {
				log.V(3).Info("LoadBalancer is still not Active", "loadBalancerName", *loadBalancer.Name, "state", *loadBalancer.ProvisioningStatus)
				isAnyLoadBalancerNotReady = true
			}

			loadBalancerStatus := infrav1.VPCLoadBalancerStatus{
				ID:       loadBalancer.ID,
				State:    infrav1.VPCLoadBalancerState(*loadBalancer.ProvisioningStatus),
				Hostname: loadBalancer.Hostname,
			}
			s.SetLoadBalancerStatus(ctx, *loadBalancer.Name, loadBalancerStatus)
			continue
		}

		// check VPC load balancer exist in cloud
		loadBalancerStatus, err := s.checkLoadBalancer(ctx, loadBalancer)
		if err != nil {
			return false, fmt.Errorf("failed to check if load balancer exists: %w", err)
		}
		if loadBalancerStatus != nil {
			log.V(3).Info("Found load balancer in cloud", "loadBalancerID", *loadBalancerStatus.ID)
			s.SetLoadBalancerStatus(ctx, loadBalancer.Name, *loadBalancerStatus)
			continue
		}

		// check load balancer port against apiserver port.
		if err := s.checkLoadBalancerPort(loadBalancer); err != nil {
			return false, fmt.Errorf("failed to check load balancer port: %w", err)
		}

		// create loadBalancer
		log.Info("Creating load balancer")
		loadBalancerStatus, err = s.createLoadBalancer(ctx, loadBalancer)
		if err != nil {
			return false, fmt.Errorf("failed to create load balancer: %w", err)
		}
		log.Info("Created load balancer", "loadBalancerID", loadBalancerStatus.ID)
		s.SetLoadBalancerStatus(ctx, loadBalancer.Name, *loadBalancerStatus)
		isAnyLoadBalancerNotReady = true
	}
	if isAnyLoadBalancerNotReady {
		return false, nil
	}
	return true, nil
}

// checkLoadBalancerStatus checks the state of a VPC load balancer.
// If state is active, true is returned, in all other cases, it returns false indicating that load balancer is still not ready.
func (s *ClusterScope) checkLoadBalancerStatus(ctx context.Context, lb vpcv1.LoadBalancer) bool {
	log := ctrl.LoggerFrom(ctx)
	log.V(3).Info("Checking the status of VPC load balancer", "loadBalancerName", *lb.Name)
	switch *lb.ProvisioningStatus {
	case string(infrav1.VPCLoadBalancerStateActive):
		log.V(3).Info("Load balancer is in active state")
		return true
	case string(infrav1.VPCLoadBalancerStateCreatePending):
		log.V(3).Info("Load balancer creation is in pending state")
	case string(infrav1.VPCLoadBalancerStateUpdatePending):
		log.V(3).Info("Load balancer is in updating state")
	}
	return false
}

func (s *ClusterScope) checkLoadBalancerPort(lb infrav1.VPCLoadBalancerSpec) error {
	for _, listener := range lb.AdditionalListeners {
		if listener.Port == int64(s.APIServerPort()) {
			return fmt.Errorf("port %d for the %s load balancer cannot be used as an additional listener port, as it is already assigned to the API server", listener.Port, lb.Name)
		}
	}
	return nil
}

// checkLoadBalancer checks if VPC load balancer by the given name exists in cloud.
func (s *ClusterScope) checkLoadBalancer(ctx context.Context, lb infrav1.VPCLoadBalancerSpec) (*infrav1.VPCLoadBalancerStatus, error) {
	log := ctrl.LoggerFrom(ctx)
	loadBalancer, err := s.IBMVPCClient.GetLoadBalancerByName(lb.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch load balancer details: %w", err)
	}
	if loadBalancer == nil {
		log.V(3).Info("VPC load balancer not found in cloud")
		return nil, nil
	}
	return &infrav1.VPCLoadBalancerStatus{
		ID:       loadBalancer.ID,
		State:    infrav1.VPCLoadBalancerState(*loadBalancer.ProvisioningStatus),
		Hostname: loadBalancer.Hostname,
	}, nil
}

// createLoadBalancer creates loadBalancer.
func (s *ClusterScope) createLoadBalancer(ctx context.Context, lb infrav1.VPCLoadBalancerSpec) (*infrav1.VPCLoadBalancerStatus, error) {
	log := ctrl.LoggerFrom(ctx)
	options := &vpcv1.CreateLoadBalancerOptions{}
	// TODO(karthik-k-n): consider moving resource group id to clusterscope
	// fetch resource group id
	resourceGroupID := s.GetResourceGroupID()
	if resourceGroupID == "" {
		return nil, fmt.Errorf("failed to fetch resource group ID for resource group %v, ID is empty", s.ResourceGroup())
	}

	var isPublic bool
	if lb.Public != nil && *lb.Public {
		isPublic = true
	}
	options.SetIsPublic(isPublic)
	options.SetName(lb.Name)
	options.SetResourceGroup(&vpcv1.ResourceGroupIdentity{
		ID: &resourceGroupID,
	})

	subnetIDs := s.GetVPCSubnetIDs()
	if subnetIDs == nil {
		return nil, fmt.Errorf("no subnets are present for load balancer creation")
	}
	for _, subnetID := range subnetIDs {
		subnet := &vpcv1.SubnetIdentity{
			ID: subnetID,
		}
		options.Subnets = append(options.Subnets, subnet)
	}
	options.SetPools([]vpcv1.LoadBalancerPoolPrototypeLoadBalancerContext{
		{
			Algorithm:     core.StringPtr("round_robin"),
			HealthMonitor: &vpcv1.LoadBalancerPoolHealthMonitorPrototype{Delay: core.Int64Ptr(5), MaxRetries: core.Int64Ptr(2), Timeout: core.Int64Ptr(2), Type: core.StringPtr("tcp")},
			// Note: Appending port number to the name, it will be referenced to set target port while adding new pool member
			Name:     core.StringPtr(fmt.Sprintf("%s-pool-%d", lb.Name, s.APIServerPort())),
			Protocol: core.StringPtr("tcp"),
		},
	})

	options.SetListeners([]vpcv1.LoadBalancerListenerPrototypeLoadBalancerContext{
		{
			Protocol: core.StringPtr("tcp"),
			Port:     core.Int64Ptr(int64(s.APIServerPort())),
			DefaultPool: &vpcv1.LoadBalancerPoolIdentityByName{
				Name: core.StringPtr(fmt.Sprintf("%s-pool-%d", lb.Name, s.APIServerPort())),
			},
		},
	})

	for _, additionalListeners := range lb.AdditionalListeners {
		pool := vpcv1.LoadBalancerPoolPrototypeLoadBalancerContext{
			Algorithm:     core.StringPtr("round_robin"),
			HealthMonitor: &vpcv1.LoadBalancerPoolHealthMonitorPrototype{Delay: core.Int64Ptr(5), MaxRetries: core.Int64Ptr(2), Timeout: core.Int64Ptr(2), Type: core.StringPtr("tcp")},
			// Note: Appending port number to the name, it will be referenced to set target port while adding new pool member
			Name:     ptr.To(fmt.Sprintf("additional-pool-%d", additionalListeners.Port)),
			Protocol: core.StringPtr("tcp"),
		}
		options.Pools = append(options.Pools, pool)

		listener := vpcv1.LoadBalancerListenerPrototypeLoadBalancerContext{
			Protocol: core.StringPtr("tcp"),
			Port:     core.Int64Ptr(additionalListeners.Port),
			DefaultPool: &vpcv1.LoadBalancerPoolIdentityByName{
				Name: ptr.To(fmt.Sprintf("additional-pool-%d", additionalListeners.Port)),
			},
		}
		options.Listeners = append(options.Listeners, listener)
	}

	log.V(5).Info("Creating load balancer", "options", options)
	loadBalancer, _, err := s.IBMVPCClient.CreateLoadBalancer(options)
	if err != nil {
		return nil, fmt.Errorf("failed to create load balancer: %w", err)
	}
	lbState := infrav1.VPCLoadBalancerState(*loadBalancer.ProvisioningStatus)
	return &infrav1.VPCLoadBalancerStatus{
		ID:                loadBalancer.ID,
		State:             lbState,
		Hostname:          loadBalancer.Hostname,
		ControllerCreated: ptr.To(true),
	}, nil
}

// COSInstance returns the COS instance reference.
func (s *ClusterScope) COSInstance() *infrav1.CosInstance {
	return s.IBMPowerVSCluster.Spec.CosInstance
}

// ReconcileCOSInstance reconcile COS bucket.
func (s *ClusterScope) ReconcileCOSInstance(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)
	// check COS service instance exist in cloud
	cosServiceInstanceStatus, err := s.checkCOSServiceInstance(ctx)
	if err != nil {
		return fmt.Errorf("failed to check if COS instance in cloud: %w", err)
	}
	if cosServiceInstanceStatus != nil {
		log.V(3).Info("COS service instance found in cloud")
		s.SetStatus(ctx, infrav1.ResourceTypeCOSInstance, infrav1.ResourceReference{ID: cosServiceInstanceStatus.GUID, ControllerCreated: ptr.To(false)})
	} else {
		// create COS service instance
		log.V(3).Info("Creating COS service instance")
		cosServiceInstanceStatus, err = s.createCOSServiceInstance()
		if err != nil {
			return fmt.Errorf("failed to create COS service instance: %w", err)
		}
		log.Info("Created COS service instance", "cosID", cosServiceInstanceStatus.GUID)
		s.SetStatus(ctx, infrav1.ResourceTypeCOSInstance, infrav1.ResourceReference{ID: cosServiceInstanceStatus.GUID, ControllerCreated: ptr.To(true)})
	}

	props, err := authenticator.GetProperties()
	if err != nil {
		return fmt.Errorf("failed to get authenticator properties: %w", err)
	}

	apiKey, ok := props["APIKEY"]
	if !ok {
		return fmt.Errorf("IBM Cloud API key is not provided, set %s environmental variable", "IBMCLOUD_API_KEY")
	}

	region := s.bucketRegion()
	if region == "" {
		return fmt.Errorf("failed to determine COS bucket region, both bucket region and VPC region not set")
	}

	serviceEndpoint := fmt.Sprintf("s3.%s.%s", region, cosURLDomain)
	// Fetch the COS service endpoint.
	cosServiceEndpoint := endpoints.FetchEndpoints(string(endpoints.COS), s.ServiceEndpoint)
	if cosServiceEndpoint != "" {
		log.V(3).Info("Overriding the default COS endpoint", "cosEndpoint", cosServiceEndpoint)
		serviceEndpoint = cosServiceEndpoint
	}

	cosOptions := cos.ServiceOptions{
		Options: &cosSession.Options{
			Config: aws.Config{
				Endpoint: &serviceEndpoint,
				Region:   &region,
			},
		},
	}

	cosClient, err := cos.NewServiceWrapper(cosOptions, apiKey, *cosServiceInstanceStatus.GUID)
	if err != nil {
		return fmt.Errorf("failed to create COS client: %w", err)
	}
	s.COSClient = cosClient

	// check bucket exist in service instance
	if exist, err := s.checkCOSBucket(); exist {
		log.V(3).Info("COS bucket found in cloud")
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to check if COS bucket exists: %w", err)
	}

	// create bucket in service instance
	if err := s.createCOSBucket(); err != nil {
		return fmt.Errorf("failed to create COS bucket: %w", err)
	}
	return nil
}

func (s *ClusterScope) checkCOSBucket() (bool, error) {
	if _, err := s.COSClient.GetBucketByName(*s.GetServiceName(infrav1.ResourceTypeCOSBucket)); err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			switch aerr.Code() {
			case s3.ErrCodeNoSuchBucket, "Forbidden", "NotFound":
				// If the bucket doesn't exist that's ok, we'll try to create it
				return false, nil
			default:
				return false, err
			}
		} else {
			return false, err
		}
	}
	return true, nil
}

func (s *ClusterScope) createCOSBucket() error {
	input := &s3.CreateBucketInput{
		Bucket: ptr.To(*s.GetServiceName(infrav1.ResourceTypeCOSBucket)),
	}
	_, err := s.COSClient.CreateBucket(input)
	if err == nil {
		return nil
	}

	aerr, ok := err.(awserr.Error)
	if !ok {
		return fmt.Errorf("failed to create COS bucket %w", err)
	}

	switch aerr.Code() {
	// If bucket already exists, all good.
	case s3.ErrCodeBucketAlreadyOwnedByYou:
		return nil
	case s3.ErrCodeBucketAlreadyExists:
		return nil
	default:
		return fmt.Errorf("failed to create COS bucket %w", err)
	}
}

func (s *ClusterScope) checkCOSServiceInstance(ctx context.Context) (*resourcecontrollerv2.ResourceInstance, error) {
	log := ctrl.LoggerFrom(ctx)
	// check cos service instance
	resourceInstance := resourcecontroller.InstanceFilter{
		Name:           *s.GetServiceName(infrav1.ResourceTypeCOSInstance),
		ResourceID:     resourcecontroller.CosResourceID,
		ResourcePlanID: resourcecontroller.CosResourcePlanID,
	}

	serviceInstance, err := s.ResourceClient.GetResourceInstanceByFilter(resourceInstance)
	if err != nil {
		return nil, fmt.Errorf("failed to get COS service instance: %w", err)
	}

	if serviceInstance == nil {
		log.V(3).Info("COS service instance is not found", "cosInstanceName", *s.GetServiceName(infrav1.ResourceTypeCOSInstance))
		return nil, nil
	}
	if *serviceInstance.State != string(infrav1.WorkspaceStateActive) {
		return nil, fmt.Errorf("COS service instance is not in active state, current state: %s", *serviceInstance.State)
	}
	return serviceInstance, nil
}

func (s *ClusterScope) createCOSServiceInstance() (*resourcecontrollerv2.ResourceInstance, error) {
	// fetch resource group id.
	resourceGroupID := s.GetResourceGroupID()
	if resourceGroupID == "" {
		return nil, fmt.Errorf("failed to fetch resource group ID for resource group %v, ID is empty", s.ResourceGroup())
	}

	target := "Global"
	// create service instance
	serviceInstance, _, err := s.ResourceClient.CreateResourceInstance(&resourcecontrollerv2.CreateResourceInstanceOptions{
		Name:           s.GetServiceName(infrav1.ResourceTypeCOSInstance),
		Target:         &target,
		ResourceGroup:  &resourceGroupID,
		ResourcePlanID: ptr.To(resourcecontroller.CosResourcePlanID),
	})
	if err != nil {
		return nil, err
	}
	return serviceInstance, nil
}

// fetchResourceGroupID retrieving id of resource group.
func (s *ClusterScope) fetchResourceGroupID() (string, error) {
	if s.ResourceGroup() == nil || s.ResourceGroup().Name == nil {
		return "", fmt.Errorf("resource group name is not set")
	}

	auth, err := authenticator.GetAuthenticator()
	if err != nil {
		return "", err
	}

	account, err := accounts.GetAccount(auth)
	if err != nil {
		return "", err
	}

	resourceGroup := s.ResourceGroup().Name
	rmv2ListResourceGroupOpt := resourcemanagerv2.ListResourceGroupsOptions{Name: resourceGroup, AccountID: &account}
	resourceGroupListResult, _, err := s.ResourceManagerClient.ListResourceGroups(&rmv2ListResourceGroupOpt)
	if err != nil {
		return "", err
	}

	if resourceGroupListResult != nil && len(resourceGroupListResult.Resources) > 0 {
		rg := resourceGroupListResult.Resources[0]
		resourceGroupID := *rg.ID
		return resourceGroupID, nil
	}

	err = fmt.Errorf("could not retrieve resource group ID for %s", *resourceGroup)
	return "", err
}

// fetchVPCCRN returns VPC CRN.
func (s *ClusterScope) fetchVPCCRN() (*string, error) {
	vpcID := s.GetVPCID()
	if vpcID == nil {
		if s.IBMPowerVSCluster.Spec.VPC != nil && s.IBMPowerVSCluster.Spec.VPC.ID != nil {
			vpcID = s.IBMPowerVSCluster.Spec.VPC.ID
		}
	}
	vpcDetails, _, err := s.IBMVPCClient.GetVPC(&vpcv1.GetVPCOptions{
		ID: vpcID,
	})
	if err != nil {
		return nil, err
	}
	return vpcDetails.CRN, nil
}

// fetchPowerVSServiceInstanceCRN returns Power VS service instance CRN.
func (s *ClusterScope) fetchPowerVSServiceInstanceCRN() (*string, error) {
	workspaceID := s.IBMPowerVSCluster.Status.Workspace.ID
	workspace, _, err := s.ResourceClient.GetResourceInstance(&resourcecontrollerv2.GetResourceInstanceOptions{
		ID: &workspaceID,
	})
	if err != nil {
		return nil, err
	}
	if workspace.CRN == nil {
		return nil, fmt.Errorf("workspace CRN is empty %s", workspaceID)
	}
	return workspace.CRN, nil
}

// TODO(karthik-k-n): Decide on proper naming format for services.

// GetServiceName returns name of given service type from spec or generate a name for it.
func (s *ClusterScope) GetServiceName(resourceType infrav1.ResourceType) *string {
	switch resourceType {
	case infrav1.ResourceTypeVPC:
		if s.VPC() == nil || s.VPC().Name == nil {
			return ptr.To(fmt.Sprintf("%s-vpc", s.InfraCluster()))
		}
		return s.VPC().Name
	case infrav1.ResourceTypeTransitGateway:
		if s.TransitGateway() == nil || s.TransitGateway().Name == nil {
			return ptr.To(fmt.Sprintf("%s-transitgateway", s.InfraCluster()))
		}
		return s.TransitGateway().Name
	case infrav1.ResourceTypeCOSInstance:
		if s.COSInstance() == nil || s.COSInstance().Name == "" {
			return ptr.To(fmt.Sprintf("%s-cosinstance", s.InfraCluster()))
		}
		return &s.COSInstance().Name
	case infrav1.ResourceTypeCOSBucket:
		if s.COSInstance() == nil || s.COSInstance().BucketName == "" {
			return ptr.To(fmt.Sprintf("%s-cosbucket", s.InfraCluster()))
		}
		return &s.COSInstance().BucketName
	case infrav1.ResourceTypeSubnet:
		return ptr.To(fmt.Sprintf("%s-vpcsubnet", s.InfraCluster()))
	case infrav1.ResourceTypeLoadBalancer:
		return ptr.To(fmt.Sprintf("%s-loadbalancer", s.InfraCluster()))
	}
	return nil
}

// DeleteLoadBalancer deletes loadBalancer.
func (s *ClusterScope) DeleteLoadBalancer(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	var errs []error
	requeue := false
	for _, lb := range s.IBMPowerVSCluster.Status.LoadBalancers {
		if lb.ID == nil || lb.ControllerCreated == nil || !*lb.ControllerCreated {
			log.Info("Skipping load balancer deletion as resource is not created by controller")
			continue
		}

		lb, resp, err := s.IBMVPCClient.GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{
			ID: lb.ID,
		})

		if err != nil {
			if resp != nil && resp.StatusCode == ResourceNotFoundCode {
				log.Info("Load balancer successfully deleted")
				continue
			}
			errs = append(errs, fmt.Errorf("failed to fetch load balancer: %w", err))
			continue
		}

		if lb != nil && lb.ProvisioningStatus != nil && *lb.ProvisioningStatus == string(infrav1.VPCLoadBalancerStateDeletePending) {
			log.V(3).Info("Load balancer is currently being deleted")
			return true, nil
		}

		if _, err = s.IBMVPCClient.DeleteLoadBalancer(&vpcv1.DeleteLoadBalancerOptions{
			ID: lb.ID,
		}); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete load balancer: %w", err))
			continue
		}
		requeue = true
	}
	if len(errs) > 0 {
		return false, kerrors.NewAggregate(errs)
	}
	return requeue, nil
}

// DeleteVPCSecurityGroups deletes VPC security group.
func (s *ClusterScope) DeleteVPCSecurityGroups(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)
	for _, securityGroup := range s.IBMPowerVSCluster.Status.VPCSecurityGroups {
		if securityGroup.ControllerCreated == nil || !*securityGroup.ControllerCreated {
			log.Info("Skipping VPC security group deletion as resource is not created by controller", "securityGroupID", *securityGroup.ID)
			continue
		}
		if _, resp, err := s.IBMVPCClient.GetSecurityGroup(&vpcv1.GetSecurityGroupOptions{
			ID: securityGroup.ID,
		}); err != nil {
			if resp != nil && resp.StatusCode == ResourceNotFoundCode {
				log.Info("VPC security group has been already deleted", "securityGroupID", *securityGroup.ID)
				continue
			}
			return fmt.Errorf("failed to fetch VPC security group '%s': %w", *securityGroup.ID, err)
		}

		log.V(3).Info("Deleting VPC security group", "securityGroupID", *securityGroup.ID)
		options := &vpcv1.DeleteSecurityGroupOptions{
			ID: securityGroup.ID,
		}
		if _, err := s.IBMVPCClient.DeleteSecurityGroup(options); err != nil {
			return fmt.Errorf("failed to delete VPC security group '%s': %w", *securityGroup.ID, err)
		}
		log.Info("VPC security group successfully deleted", "securityGroupID", *securityGroup.ID)
	}
	return nil
}

// DeleteVPCSubnet deletes VPC subnet.
func (s *ClusterScope) DeleteVPCSubnet(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	var errs []error
	requeue := false
	for _, subnet := range s.IBMPowerVSCluster.Status.VPCSubnet {
		if subnet.ID == nil || subnet.ControllerCreated == nil || !*subnet.ControllerCreated {
			log.Info("Skipping VPC subnet deletion as resource is not created by controller")
			continue
		}

		net, resp, err := s.IBMVPCClient.GetSubnet(&vpcv1.GetSubnetOptions{
			ID: subnet.ID,
		})

		if err != nil {
			if resp != nil && resp.StatusCode == ResourceNotFoundCode {
				log.Info("VPC subnet successfully deleted")
				continue
			}
			errs = append(errs, fmt.Errorf("failed to fetch VPC subnet: %w", err))
			continue
		}

		if net != nil && net.Status != nil && *net.Status == string(infrav1.VPCSubnetStateDeleting) {
			return true, nil
		}

		if _, err = s.IBMVPCClient.DeleteSubnet(&vpcv1.DeleteSubnetOptions{
			ID: net.ID,
		}); err != nil {
			errs = append(errs, fmt.Errorf("failed to delete VPC subnet: %w", err))
			continue
		}
		requeue = true
	}
	if len(errs) > 0 {
		return false, kerrors.NewAggregate(errs)
	}
	return requeue, nil
}

// DeleteVPC deletes VPC.
func (s *ClusterScope) DeleteVPC(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	if !s.isResourceCreatedByController(infrav1.ResourceTypeVPC) {
		log.Info("Skipping VPC deletion as resource is not created by controller")
		return false, nil
	}

	if s.IBMPowerVSCluster.Status.VPC.ID == nil {
		return false, nil
	}

	vpcDetails, resp, err := s.IBMVPCClient.GetVPC(&vpcv1.GetVPCOptions{
		ID: s.IBMPowerVSCluster.Status.VPC.ID,
	})

	if err != nil {
		if resp != nil && resp.StatusCode == ResourceNotFoundCode {
			log.Info("VPC successfully deleted")
			return false, nil
		}
		return false, fmt.Errorf("failed to fetch VPC: %w", err)
	}

	if vpcDetails != nil && vpcDetails.Status != nil && *vpcDetails.Status == string(infrav1.VPCStateDeleting) {
		return true, nil
	}

	if _, err = s.IBMVPCClient.DeleteVPC(&vpcv1.DeleteVPCOptions{
		ID: vpcDetails.ID,
	}); err != nil {
		return false, fmt.Errorf("failed to delete VPC: %w", err)
	}
	return true, nil
}

// DeleteTransitGateway deletes transit gateway.
func (s *ClusterScope) DeleteTransitGateway(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	skipTGDeletion := false
	if !s.isResourceCreatedByController(infrav1.ResourceTypeTransitGateway) {
		log.Info("Skipping transit gateway deletion as resource is not created by controller, but will check if connections are created by the controller")
		skipTGDeletion = true
	}

	if s.IBMPowerVSCluster.Status.TransitGateway == nil {
		return false, nil
	}

	tg, resp, err := s.TransitGatewayClient.GetTransitGateway(&tgapiv1.GetTransitGatewayOptions{
		ID: s.IBMPowerVSCluster.Status.TransitGateway.ID,
	})

	if err != nil {
		if resp != nil && resp.StatusCode == ResourceNotFoundCode {
			log.Info("Transit gateway successfully deleted")
			return false, nil
		}
		return false, fmt.Errorf("failed to fetch transit gateway: %w", err)
	}

	if tg.Status != nil && *tg.Status == string(infrav1.TransitGatewayStateDeletePending) {
		log.V(3).Info("Transit gateway is being deleted")
		return true, nil
	}

	requeue, err := s.deleteTransitGatewayConnections(ctx, tg)
	if err != nil {
		return false, err
	} else if requeue {
		return true, nil
	}

	if skipTGDeletion {
		return false, nil
	}

	if _, err = s.TransitGatewayClient.DeleteTransitGateway(&tgapiv1.DeleteTransitGatewayOptions{
		ID: s.IBMPowerVSCluster.Status.TransitGateway.ID,
	}); err != nil {
		return false, fmt.Errorf("failed to delete transit gateway: %w", err)
	}
	return true, nil
}

func (s *ClusterScope) deleteTransitGatewayConnections(ctx context.Context, tg *tgapiv1.TransitGateway) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	deleteConnection := func(connID *string) (bool, error) {
		conn, resp, err := s.TransitGatewayClient.GetTransitGatewayConnection(&tgapiv1.GetTransitGatewayConnectionOptions{
			TransitGatewayID: tg.ID,
			ID:               connID,
		})
		if resp.StatusCode == ResourceNotFoundCode {
			log.V(3).Info("Connection deleted in transit gateway", "connectionID", *connID)
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("failed to get transit gateway powervs connection: %w", err)
		}
		if conn.Status != nil && *conn.Status == string(infrav1.TransitGatewayConnectionStateDeleting) {
			log.V(3).Info("Transit gateway connection is in deleting state")
			return true, nil
		}

		if _, err = s.TransitGatewayClient.DeleteTransitGatewayConnection(&tgapiv1.DeleteTransitGatewayConnectionOptions{
			ID:               connID,
			TransitGatewayID: tg.ID,
		}); err != nil {
			return false, fmt.Errorf("failed to delete transit gateway connection: %w", err)
		}

		return true, nil
	}
	if *s.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ControllerCreated {
		log.V(3).Info("Deleting PowerVS connection in Transit gateway")
		requeue, err := deleteConnection(s.IBMPowerVSCluster.Status.TransitGateway.PowerVSConnection.ID)
		if err != nil {
			return false, err
		}
		if requeue {
			return requeue, nil
		}
	}

	if *s.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ControllerCreated {
		log.V(3).Info("Deleting VPC connection in Transit gateway")
		requeue, err := deleteConnection(s.IBMPowerVSCluster.Status.TransitGateway.VPCConnection.ID)
		if err != nil {
			return false, err
		}
		if requeue {
			return requeue, nil
		}
	}

	return false, nil
}

// DeleteDHCPServer deletes the DHCP server if it was provisioned by the controller.
func (s *ClusterScope) DeleteDHCPServer(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)

	// 1. Check if we provisioned this network
	if s.IBMPowerVSCluster.Spec.Network.Type != infrav1.SourceTypeProvision {
		log.Info("Skipping DHCP server deletion as network is in Reference mode")
		return nil
	}

	// 2. If the controller owns the workspace, deleting the workspace cascades
	// and destroys the DHCP server internally
	if s.IBMPowerVSCluster.Spec.Workspace.Type == infrav1.SourceTypeProvision {
		log.Info("Skipping separate DHCP server deletion as PowerVS workspace is being deleted by the controller (cascading delete)")
		return nil
	}

	// 3. Get DHCP server ID saved in status
	dhcpID := s.IBMPowerVSCluster.Status.Network.DHCPServer.ID
	if dhcpID == "" {
		log.Info("DHCP server ID not found in status, nothing to delete")
		return nil
	}

	// 4. Fetch the server to verify it exists
	server, err := s.IBMPowerVSClient.GetDHCPServer(dhcpID)
	if err != nil {
		// If it's a 404, we're already done!
		if strings.Contains(err.Error(), string(DHCPServerNotFound)) {
			log.Info("DHCP server no longer exists in IBM Cloud")
			return nil
		}
		return fmt.Errorf("failed to fetch DHCP server: %w", err)
	}

	// 5. Issue the delete command
	log.Info("Deleting provisioned DHCP server", "dhcpServerID", *server.ID)
	if err = s.IBMPowerVSClient.DeleteDHCPServer(*server.ID); err != nil {
		return fmt.Errorf("failed to delete DHCP server: %w", err)
	}

	return nil
}

// DeleteWorkspace deletes the PowerVS workspace if it was provisioned by the controller.
func (s *ClusterScope) DeleteWorkspace(ctx context.Context) (bool, error) {
	log := ctrl.LoggerFrom(ctx)
	cluster := s.IBMPowerVSCluster

	// 1. Declarative Safety Check: Only delete if the user asked us to provision it.
	if cluster.Spec.Workspace.Type != infrav1.SourceTypeProvision {
		log.Info("Skipping PowerVS workspace deletion as it was not provisioned by the controller")
		return false, nil
	}

	// 2. Check if there is actually an ID to delete.
	workspaceID := cluster.Status.Workspace.ID
	if workspaceID == "" {
		log.Info("PowerVS workspace ID is empty, nothing to delete")
		return false, nil
	}

	// 3. Fetch the current state of the workspace from IBM Cloud.
	workspace, _, err := s.ResourceClient.GetResourceInstance(&resourcecontrollerv2.GetResourceInstanceOptions{
		ID: &workspaceID,
	})
	if err != nil {
		return false, fmt.Errorf("failed to fetch PowerVS workspace: %w", err)
	}

	// 4. Check if it is already removed.
	if workspace != nil && workspace.State != nil && *workspace.State == string(infrav1.WorkspaceStateRemoved) {
		log.Info("PowerVS workspace has been removed")
		return false, nil
	}

	// 5. Trigger the deletion.
	log.Info("Deleting PowerVS workspace", "workspaceID", workspaceID)
	if _, err = s.ResourceClient.DeleteResourceInstance(&resourcecontrollerv2.DeleteResourceInstanceOptions{
		ID: &workspaceID,
	}); err != nil {
		return false, fmt.Errorf("failed to delete PowerVS workspace: %w", err)
	}

	// Return true to requeue so the controller can verify the deletion completed in the next loop.
	return true, nil
}

// DeleteCOSInstance deletes COS instance.
func (s *ClusterScope) DeleteCOSInstance(ctx context.Context) error {
	log := ctrl.LoggerFrom(ctx)
	if !s.isResourceCreatedByController(infrav1.ResourceTypeCOSInstance) {
		log.Info("Skipping COS instance deletion as resource is not created by controller")
		return nil
	}

	if s.IBMPowerVSCluster.Status.COSInstance.ID == nil {
		return nil
	}

	cosInstance, resp, err := s.ResourceClient.GetResourceInstance(&resourcecontrollerv2.GetResourceInstanceOptions{
		ID: s.IBMPowerVSCluster.Status.COSInstance.ID,
	})
	if err != nil {
		if resp != nil && resp.StatusCode == ResourceNotFoundCode {
			return nil
		}
		return fmt.Errorf("failed to fetch COS service instance: %w", err)
	}

	if cosInstance != nil && (*cosInstance.State == "pending_reclamation" || *cosInstance.State == string(infrav1.WorkspaceStateRemoved)) {
		log.Info("COS service instance has been removed")
		return nil
	}

	if _, err = s.ResourceClient.DeleteResourceInstance(&resourcecontrollerv2.DeleteResourceInstanceOptions{
		ID:        cosInstance.ID,
		Recursive: ptr.To(true),
	}); err != nil {
		log.Error(err, "failed to delete COS service instance")
		return err
	}
	log.Info("COS service instance successfully deleted")
	return nil
}

// resourceCreatedByController helps to identify resource created by controller or not.
func (s *ClusterScope) isResourceCreatedByController(resourceType infrav1.ResourceType) bool {
	switch resourceType {
	case infrav1.ResourceTypeVPC:
		vpcStatus := s.IBMPowerVSCluster.Status.VPC
		if vpcStatus == nil || vpcStatus.ControllerCreated == nil || !*vpcStatus.ControllerCreated {
			return false
		}
		return true
	case infrav1.ResourceTypeTransitGateway:
		transitGateway := s.IBMPowerVSCluster.Status.TransitGateway
		if transitGateway == nil || transitGateway.ControllerCreated == nil || !*transitGateway.ControllerCreated {
			return false
		}
		return true
	case infrav1.ResourceTypeCOSInstance:
		cosInstance := s.IBMPowerVSCluster.Status.COSInstance
		if cosInstance == nil || cosInstance.ControllerCreated == nil || !*cosInstance.ControllerCreated {
			return false
		}
		return true
	}
	return false
}

// bucketRegion returns the region to use for COS bucket for the ClusterScope.
func (s *ClusterScope) bucketRegion() string {
	return fetchBucketRegion(s.COSInstance(), s.VPC())
}
