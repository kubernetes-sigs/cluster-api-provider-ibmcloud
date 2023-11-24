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

package scope

import (
	"context"
	"errors"
	"fmt"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/globalcatalog"

	"github.com/go-logr/logr"

	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	tgapiv1 "github.com/IBM/networking-go-sdk/transitgatewayapisv1"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/IBM/platform-services-go-sdk/resourcemanagerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"

	"k8s.io/utils/pointer"
	"sigs.k8s.io/controller-runtime/pkg/client"

	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"

	infrav1beta2 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta2"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/authenticator"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/transitgateway"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/vpc"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/endpoints"
	genUtil "sigs.k8s.io/cluster-api-provider-ibmcloud/util"
)

const (
	// DEBUGLEVEL indicates the debug level of the logs.
	DEBUGLEVEL = 5
	// PowerVS service and plan name
	powerVSService     = "power-iaas"
	powerVSServicePlan = "power-virtual-server-group"
)

// ResourceType describes IBM Cloud resource name.
type ResourceType string

var (
	// ServiceInstance is Power VS service instance resource.
	ServiceInstance = ResourceType("serviceInstance")
	// Network is Power VS network resource.
	Network = ResourceType("network")
	// LoadBalancer VPC loadBalancer resource.
	LoadBalancer = ResourceType("loadBalancer")
	// TransitGateway is transit gateway resource.
	TransitGateway = ResourceType("transitGateway")
	// VPC is Power VS network resource.
	VPC = ResourceType("vpc")
	// Subnet VPC subnet resource.
	Subnet = ResourceType("subnet")
)

// PowerVSClusterScopeParams defines the input parameters used to create a new PowerVSClusterScope.
type PowerVSClusterScopeParams struct {
	Client            client.Client
	Logger            logr.Logger
	Cluster           *capiv1beta1.Cluster
	IBMPowerVSCluster *infrav1beta2.IBMPowerVSCluster
	ServiceEndpoint   []endpoints.ServiceEndpoint
}

// PowerVSClusterScope defines a scope defined around a Power VS Cluster.
type PowerVSClusterScope struct {
	logr.Logger
	Client      client.Client
	patchHelper *patch.Helper
	session     *ibmpisession.IBMPISession

	IBMPowerVSClient     powervs.PowerVS
	IBMVPCClient         vpc.Vpc
	TransitGatewayClient transitgateway.TransitGateway
	ResourceClient       resourcecontroller.ResourceController
	CatalogClient        globalcatalog.GlobalCatalog

	Cluster           *capiv1beta1.Cluster
	IBMPowerVSCluster *infrav1beta2.IBMPowerVSCluster
	ServiceEndpoint   []endpoints.ServiceEndpoint
}

// ClusterObject represents a IBMPowerVS cluster object.
type ClusterObject interface {
	conditions.Setter
}

// NewPowerVSClusterScope creates a new PowerVSClusterScope from the supplied parameters.
func NewPowerVSClusterScope(params PowerVSClusterScopeParams) (*PowerVSClusterScope, error) {
	if params.Client == nil {
		err := errors.New("failed to generate new scope from nil Client")
		return nil, err
	}
	if params.Cluster == nil {
		err := errors.New("failed to generate new scope from nil Cluster")
		return nil, err
	}
	if params.IBMPowerVSCluster == nil {
		err := errors.New("failed to generate new scope from nil IBMPowerVSCluster")
		return nil, err
	}

	helper, err := patch.NewHelper(params.IBMPowerVSCluster, params.Client)
	if err != nil {
		err = fmt.Errorf("failed to init patch helper: %w", err)
		return nil, err
	}

	auth, err := authenticator.GetAuthenticator()
	if err != nil {
		return nil, err
	}
	account, err := utils.GetAccount(auth)
	if err != nil {
		return nil, err
	}
	// TODO(Karthik-k-n): Handle dubug and URL options.
	sessionOptions := &ibmpisession.IBMPIOptions{
		Authenticator: auth,
		UserAccount:   account,
		Zone:          *params.IBMPowerVSCluster.Spec.Zone,
	}
	session, err := ibmpisession.NewIBMPISession(sessionOptions)
	if err != nil {
		return nil, err
	}
	// TODO(karhtik-k-n): may be optimize NewService to use the session created here
	powerVSClient, err := powervs.NewService(powervs.ServiceOptions{})
	if err != nil {
		return nil, err
	}

	svcEndpoint := endpoints.FetchVPCEndpoint(genUtil.ConstructVPCRegionFromZone(*params.IBMPowerVSCluster.Spec.VPC.Zone), params.ServiceEndpoint)
	vpcClient, err := vpc.NewService(svcEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to create IBM VPC client: %w", err)
	}

	tgClient, err := transitgateway.NewService()
	if err != nil {
		return nil, fmt.Errorf("failed to create tranist gateway client: %w", err)
	}

	// TODO(karthik-k-n): consider passing auth in options to resource controller
	resourceClient, err := resourcecontroller.NewService(resourcecontroller.ServiceOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create resource client: %w", err)
	}

	catalogClient, err := globalcatalog.NewService(globalcatalog.ServiceOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create catalog client: %w", err)
	}

	clusterScope := &PowerVSClusterScope{
		session:              session,
		Logger:               params.Logger,
		Client:               params.Client,
		patchHelper:          helper,
		Cluster:              params.Cluster,
		IBMPowerVSCluster:    params.IBMPowerVSCluster,
		ServiceEndpoint:      params.ServiceEndpoint,
		IBMPowerVSClient:     powerVSClient,
		IBMVPCClient:         vpcClient,
		TransitGatewayClient: tgClient,
		ResourceClient:       resourceClient,
		CatalogClient:        catalogClient,
	}
	return clusterScope, nil
}

// PatchObject persists the cluster configuration and status.
func (s *PowerVSClusterScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.IBMPowerVSCluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *PowerVSClusterScope) Close() error {
	return s.PatchObject()
}

// Name returns the CAPI cluster name.
func (s *PowerVSClusterScope) Name() string {
	return s.Cluster.Name
}

// Zone returns the cluster zone.
func (s *PowerVSClusterScope) Zone() *string {
	return s.IBMPowerVSCluster.Spec.Zone
}

// VPCZone returns the cluster VPC zone.
func (s *PowerVSClusterScope) VPCZone() *string {
	if s.IBMPowerVSCluster.Spec.VPC == nil || s.IBMPowerVSCluster.Spec.VPC.Zone == nil {
		return nil
	}
	return s.IBMPowerVSCluster.Spec.VPC.Zone
}

// ResourceGroup returns the cluster resource group.
func (s *PowerVSClusterScope) ResourceGroup() *string {
	return s.IBMPowerVSCluster.Spec.ResourceGroup
}

// InfraCluster returns the IBMPowerVS infrastructure cluster object.
func (s *PowerVSClusterScope) InfraCluster() ClusterObject {
	return s.IBMPowerVSCluster
}

// APIServerPort returns the APIServerPort to use when creating the ControlPlaneEndpoint.
func (s *PowerVSClusterScope) APIServerPort() int32 {
	if s.Cluster.Spec.ClusterNetwork != nil && s.Cluster.Spec.ClusterNetwork.APIServerPort != nil {
		return *s.Cluster.Spec.ClusterNetwork.APIServerPort
	}
	return 6443
}

// ServiceInstance returns the cluster ServiceInstance.
func (s *PowerVSClusterScope) ServiceInstance() *infrav1beta2.IBMPowerVSResourceReference {
	return s.IBMPowerVSCluster.Spec.ServiceInstance
}

// SetServiceInstanceID set the service instance id.
func (s *PowerVSClusterScope) SetServiceInstanceID(serviceInstanceID string) {
	s.IBMPowerVSCluster.Status.ServiceInstanceID = &serviceInstanceID
}

// GetServiceInstanceID get the service instance id.
func (s *PowerVSClusterScope) GetServiceInstanceID() string {
	if s.IBMPowerVSCluster.Spec.ServiceInstance.ID != nil {
		return *s.IBMPowerVSCluster.Spec.ServiceInstance.ID
	}
	if s.IBMPowerVSCluster.Status.ServiceInstanceID != nil {
		return *s.IBMPowerVSCluster.Status.ServiceInstanceID
	}
	return ""
}

// Network returns the cluster Network.
func (s *PowerVSClusterScope) Network() infrav1beta2.IBMPowerVSResourceReference {
	return s.IBMPowerVSCluster.Spec.Network
}

// SetNetworkID set the network id.
func (s *PowerVSClusterScope) SetNetworkID(networkID string) {
	s.IBMPowerVSCluster.Status.NetworkID = &networkID
}

// SetDHCPServerID set the DHCP id.
func (s *PowerVSClusterScope) SetDHCPServerID(dhcpServerID *string) {
	s.IBMPowerVSCluster.Status.DHCPServerID = dhcpServerID
}

// GetDHCPServerID returns the DHCP id.
func (s *PowerVSClusterScope) GetDHCPServerID() *string {
	return s.IBMPowerVSCluster.Status.DHCPServerID
}

// VPC returns the cluster VPC information.
func (s *PowerVSClusterScope) VPC() *infrav1beta2.VPCResourceReference {
	return s.IBMPowerVSCluster.Spec.VPC
}

// SetVPCID set the network id.
func (s *PowerVSClusterScope) SetVPCID(vpcID string) {
	s.IBMPowerVSCluster.Status.VPCID = &vpcID
}

// GetVPCID returns the VPC id.
func (s *PowerVSClusterScope) GetVPCID() *string {
	if s.IBMPowerVSCluster.Spec.VPC != nil && s.IBMPowerVSCluster.Spec.VPC.ID != nil {
		return s.IBMPowerVSCluster.Spec.VPC.ID
	}
	if s.IBMPowerVSCluster.Status.VPCID != nil {
		return s.IBMPowerVSCluster.Status.VPCID
	}
	return nil
}

// VPCSubnet returns the cluster VPC subnet information.
func (s *PowerVSClusterScope) VPCSubnet() *infrav1beta2.Subnet {
	return s.IBMPowerVSCluster.Spec.VPCSubnet
}

// GetVPCSubnetID returns the VPC subnet id.
func (s *PowerVSClusterScope) GetVPCSubnetID() *string {
	if s.IBMPowerVSCluster.Spec.VPCSubnet != nil && s.IBMPowerVSCluster.Spec.VPCSubnet.ID != nil {
		return s.IBMPowerVSCluster.Spec.VPCSubnet.ID
	}
	if s.IBMPowerVSCluster.Status.VPCID != nil {
		return s.IBMPowerVSCluster.Status.VPCSubnetID
	}
	return nil
}

// SetVPCSubnetID set the VPC subnet id.
func (s *PowerVSClusterScope) SetVPCSubnetID(subnetID string) {
	s.IBMPowerVSCluster.Status.VPCSubnetID = &subnetID
}

// TransitGateway returns the cluster Transit Gateway information.
func (s *PowerVSClusterScope) TransitGateway() *infrav1beta2.TransitGateway {
	return s.IBMPowerVSCluster.Spec.TransitGateway
}

// SetTransitGatewayID set the transit gateway id.
func (s *PowerVSClusterScope) SetTransitGatewayID(tgID string) {
	s.IBMPowerVSCluster.Status.TransitGatewayID = &tgID
}

// GetTransitGatewayID returns the transit gateway id.
func (s *PowerVSClusterScope) GetTransitGatewayID() *string {
	if s.IBMPowerVSCluster.Spec.TransitGateway != nil && s.IBMPowerVSCluster.Spec.TransitGateway.ID != nil {
		return s.IBMPowerVSCluster.Spec.TransitGateway.ID
	}
	if s.IBMPowerVSCluster.Status.TransitGatewayID != nil {
		return s.IBMPowerVSCluster.Status.TransitGatewayID
	}
	return nil
}

// LoadBalancer returns the cluster loadBalancer information.
func (s *PowerVSClusterScope) LoadBalancer() *infrav1beta2.VPCLoadBalancerSpec {
	return s.IBMPowerVSCluster.Spec.ControlPlaneLoadBalancer
}

// SetLoadBalancerStatus set the loadBalancer id.
func (s *PowerVSClusterScope) SetLoadBalancerStatus(loadBalancer *infrav1beta2.VPCLoadBalancerStatus) {
	s.IBMPowerVSCluster.Status.ControlPlaneLoadBalancer = loadBalancer
}

// GetLoadBalancerID returns the loadBalancer.
func (s *PowerVSClusterScope) GetLoadBalancerID() *string {
	if s.IBMPowerVSCluster.Status.ControlPlaneLoadBalancer != nil && s.IBMPowerVSCluster.Status.ControlPlaneLoadBalancer.ID != nil {
		return s.IBMPowerVSCluster.Status.ControlPlaneLoadBalancer.ID
	}
	return nil
}

// GetLoadBalancerState will get the state for the load balancer.
func (s *PowerVSClusterScope) GetLoadBalancerState() *infrav1beta2.VPCLoadBalancerState {
	return s.IBMPowerVSCluster.Status.ControlPlaneLoadBalancer.State
}

// GetLoadBalancerHost will return the hostname of load balancer.
func (s *PowerVSClusterScope) GetLoadBalancerHost() *string {
	return s.IBMPowerVSCluster.Status.ControlPlaneLoadBalancer.Hostname
}

// ReconcileServiceInstance reconciles service instance.
func (s *PowerVSClusterScope) ReconcileServiceInstance() error {
	if s.GetServiceInstanceID() != "" {
		serviceInstanceID := s.GetServiceInstanceID()
		serviceInstance, _, err := s.ResourceClient.GetResourceInstance(&resourcecontrollerv2.GetResourceInstanceOptions{
			ID: &serviceInstanceID,
		})
		if err != nil {
			return err
		}
		if serviceInstance == nil {
			return fmt.Errorf("error failed to get service instance with id %s", serviceInstanceID)
		}
		if *serviceInstance.State != "active" {
			return fmt.Errorf("service instance not in active state, current state: %s", *serviceInstance.State)
		}
		return nil
	}

	// check service instance exist in cloud
	serviceInstanceID, err := s.checkServiceInstance()
	if err != nil {
		return err
	}
	if serviceInstanceID != "" {
		s.SetServiceInstanceID(serviceInstanceID)
		return nil
	}

	// create Service Instance
	serviceInstance, err := s.createServiceInstance()
	if err != nil {
		return err
	}
	s.SetServiceInstanceID(*serviceInstance.GUID)
	return nil
}

// checkServiceInstance checks service instance exist in cloud.
func (s *PowerVSClusterScope) checkServiceInstance() (string, error) {
	// TODO(Karthik-k-n) support ID and Regex
	serviceInstance, err := s.ResourceClient.GetServiceInstanceByName(*s.getServiceName("serviceInstance"))
	if err != nil {
		return "", err
	}
	if serviceInstance == nil {
		return "", nil
	}
	if *serviceInstance.State != "active" {
		return "", fmt.Errorf("service instance not in active state, current state: %s", *serviceInstance.State)
	}
	return *serviceInstance.GUID, nil
}

// createServiceInstance creates the service instance.
func (s *PowerVSClusterScope) createServiceInstance() (*resourcecontrollerv2.ResourceInstance, error) {
	resourceGroupID, err := s.getResourceGroupID()
	if err != nil {
		return nil, fmt.Errorf("error getting id for resource group %s, %w", *s.ResourceGroup(), err)
	}

	_, servicePlanID, err := s.CatalogClient.GetServiceInfo(powerVSService, powerVSServicePlan)
	if err != nil {
		return nil, fmt.Errorf("error retrieving id info for powervs service %w", err)
	}

	// create service instance
	serviceInstance, _, err := s.ResourceClient.CreateResourceInstance(&resourcecontrollerv2.CreateResourceInstanceOptions{
		Name:           s.getServiceName("serviceInstance"),
		Target:         s.Zone(),
		ResourceGroup:  &resourceGroupID,
		ResourcePlanID: &servicePlanID,
	})
	if err != nil {
		return nil, err
	}
	return serviceInstance, nil
}

// ReconcileNetwork reconciles network.
func (s *PowerVSClusterScope) ReconcileNetwork() error {
	// if DHCP server id is set means the server is already created
	if s.GetDHCPServerID() != nil {
		err := s.checkDHCPServerStatus()
		if err != nil {
			return err
		}
		return nil
	}
	// check network exist in cloud
	networkID, err := s.checkNetwork()
	if err != nil {
		return err
	}
	if networkID != "" {
		s.SetNetworkID(networkID)
		return nil
	}

	dhcpServer, err := s.createDHCPServer()
	if err != nil {
		return err
	}
	if dhcpServer != nil {
		s.SetDHCPServerID(dhcpServer)
		return nil
	}

	err = s.checkDHCPServerStatus()
	if err != nil {
		return err
	}
	return nil
}

// checkNetwork checks the network exist in cloud.
func (s *PowerVSClusterScope) checkNetwork() (string, error) {
	// TODO(Karthik-k-n) support ID and Regex
	network, err := s.IBMPowerVSClient.GetNetworkByName(*s.getServiceName("network"))
	if err != nil {
		return "", err
	}
	return *network, nil
}

// checkDHCPServerStatus checks the DHCP server status.
func (s *PowerVSClusterScope) checkDHCPServerStatus() error {
	dhcpID := *s.GetDHCPServerID()
	dhcpServer, err := s.IBMPowerVSClient.GetDHCPServer(dhcpID)
	if err != nil {
		return err
	}
	if dhcpServer == nil {
		return fmt.Errorf("error failed to get dchp server")
	}
	if *dhcpServer.Status != "ACTIVE" {
		return fmt.Errorf("error dhcp server state is not active, current state %s", *dhcpServer.Status)
	}
	if dhcpServer.Network != nil && dhcpServer.Network.ID != nil {
		s.SetNetworkID(*dhcpServer.Network.ID)
	}
	return nil
}

// createDHCPServer creates the DHCP server.
func (s *PowerVSClusterScope) createDHCPServer() (*string, error) {
	dhcpServer, err := s.IBMPowerVSClient.CreateDHCPServer(&models.DHCPServerCreate{
		DNSServer: pointer.String("1.1.1.1"),
		Name:      s.getServiceName("dhcp"),
	})
	if err != nil {
		return nil, err
	}
	if dhcpServer == nil {
		return nil, fmt.Errorf("created dhcp server is nil")
	}
	return dhcpServer.ID, nil
}

// ReconcileVPC reconciles VPC.
func (s *PowerVSClusterScope) ReconcileVPC() error {
	// if VPC server id is set means the VPC is already created
	if s.GetVPCID() != nil {
		vpcDetails, _, err := s.IBMVPCClient.GetVPC(&vpcv1.GetVPCOptions{
			ID: s.GetVPCID(),
		})
		if err != nil {
			return err
		}
		if vpcDetails == nil {
			return fmt.Errorf("error failed to get vpc with id %s", *s.GetVPCID())
		}
		return nil
	}

	// check vpc exist in cloud
	vpcID, err := s.checkVPC()
	if err != nil {
		return err
	}
	if vpcID != "" {
		s.SetVPCID(vpcID)
		return nil
	}

	// TODO(karthik-k-n): create a generic cluster scope/service and implement common vpc logics, which can be consumed by both vpc and powervs

	// create VPC
	vpcDetails, err := s.createVPC()
	if err != nil {
		return err
	}
	s.SetVPCID(*vpcDetails)
	return nil
}

// checkVPC checks VPC exist in cloud.
func (s *PowerVSClusterScope) checkVPC() (string, error) {
	vpc, err := s.IBMVPCClient.GetVPCByName(*s.getServiceName("vpc"))
	if err != nil {
		return "", err
	}
	if vpc == nil {
		return "", nil
	}
	return *vpc.ID, nil
}

// createVPC creates VPC.
func (s *PowerVSClusterScope) createVPC() (*string, error) {
	resourceGroupID, err := s.getResourceGroupID()
	if err != nil {
		return nil, fmt.Errorf("error getting id for resource group %s, %w", *s.ResourceGroup(), err)
	}
	addressPrefixManagement := "auto"
	vpcOption := &vpcv1.CreateVPCOptions{
		ResourceGroup:           &vpcv1.ResourceGroupIdentity{ID: &resourceGroupID},
		Name:                    s.getServiceName("vpc"),
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
	_, _, err = s.IBMVPCClient.CreateSecurityGroupRule(options)
	if err != nil {
		return nil, err
	}
	return vpcDetails.ID, nil
}

// ReconcileVPCSubnet reconciles VPC subnet.
func (s *PowerVSClusterScope) ReconcileVPCSubnet() error {
	if s.GetVPCSubnetID() != nil {
		subnet, _, err := s.IBMVPCClient.GetSubnet(&vpcv1.GetSubnetOptions{
			ID: s.GetVPCSubnetID(),
		})
		if err != nil {
			return err
		}
		if subnet == nil {
			return fmt.Errorf("error failed to get vpc subneet with id %s", *s.GetVPCID())
		}
		return nil
	}
	// check VPC subnet exist in cloud
	vpcSubnetID, err := s.checkVPCSubnet()
	if err != nil {
		return err
	}
	if vpcSubnetID != "" {
		s.SetVPCSubnetID(vpcSubnetID)
		return nil
	}

	subnetID, err := s.createVPCSubnet()
	if err != nil {
		return err
	}
	if subnetID != nil {
		s.SetVPCSubnetID(*subnetID)
		return nil
	}
	// TODO(karthik-k-n)(Doubt): Do we need to create public gateway?
	return nil
}

// checkVPCSubnet checks VPC subnet exist in cloud.
func (s *PowerVSClusterScope) checkVPCSubnet() (string, error) {
	// TODO(karthik-k-n): Support ID
	vpcSubnet, err := s.IBMVPCClient.GetVPCSubnetByName(*s.getServiceName("vpcSubnet"))
	if err != nil {
		return "", err
	}
	if vpcSubnet == nil {
		return "", nil
	}
	return *vpcSubnet.ID, nil
}

// createVPCSubnet creates a VPC subnet.
func (s *PowerVSClusterScope) createVPCSubnet() (*string, error) {
	// TODO(karthik-k-n): consider moving to clusterscope
	// fetch resource group id
	resourceGroupID, err := s.getResourceGroupID()
	if err != nil {
		return nil, fmt.Errorf("error getting id for resource group %s, %w", *s.ResourceGroup(), err)
	}

	// create subnet
	vpcID := s.GetVPCID()
	options := &vpcv1.CreateSubnetOptions{}
	cidrBlock, err := s.IBMVPCClient.GetSubnetAddrPrefix(*vpcID, *s.IBMPowerVSCluster.Spec.Zone)
	if err != nil {
		return nil, err
	}

	options.SetSubnetPrototype(&vpcv1.SubnetPrototype{
		Ipv4CIDRBlock: &cidrBlock,
		Name:          s.getServiceName("vpcSubnet"),
		VPC: &vpcv1.VPCIdentity{
			ID: vpcID,
		},
		Zone: &vpcv1.ZoneIdentity{
			Name: s.IBMPowerVSCluster.Spec.Zone,
		},
		ResourceGroup: &vpcv1.ResourceGroupIdentity{
			ID: &resourceGroupID,
		},
	})
	subnet, _, err := s.IBMVPCClient.CreateSubnet(options)
	if err != nil {
		return nil, err
	}
	if subnet == nil {
		return nil, fmt.Errorf("subnet is nil")
	}
	return subnet.ID, nil
}

// ReconcileTransitGateway reconcile transit gateway.
func (s *PowerVSClusterScope) ReconcileTransitGateway() error {
	if s.GetTransitGatewayID() != nil {
		tg, _, err := s.TransitGatewayClient.GetTransitGateway(&tgapiv1.GetTransitGatewayOptions{
			ID: s.GetTransitGatewayID(),
		})
		if err != nil {
			return err
		}
		err = s.checkTransitGatewayStatus(tg.ID)
		if err != nil {
			return err
		}
		return nil
	}

	// check vpc exist in cloud
	tgID, err := s.checkTransitGateway()
	if err != nil {
		return err
	}
	if tgID != "" {
		s.SetTransitGatewayID(tgID)
		return nil
	}
	// create transit gateway
	transitGatewayID, err := s.createTransitGateway()
	if err != nil {
		return err
	}
	if transitGatewayID != nil {
		s.SetTransitGatewayID(*transitGatewayID)
		return nil
	}

	// verify that the transit gateway connections are attached
	err = s.checkTransitGatewayStatus(transitGatewayID)
	if err != nil {
		return err
	}
	return nil
}

// checkTransitGateway checks transit gateway exist in cloud.
func (s *PowerVSClusterScope) checkTransitGateway() (string, error) {
	// TODO(karthik-k-n): Support ID
	transitGateway, err := s.TransitGatewayClient.GetTransitGatewayByName(*s.getServiceName("transitGateway"))
	if err != nil {
		return "", err
	}
	if transitGateway == nil {
		return "", nil
	}
	if err = s.checkTransitGatewayStatus(transitGateway.ID); err != nil {
		return "", err
	}
	return *transitGateway.ID, nil
}

// checkTransitGatewayStatus checks transit gateway status in cloud.
func (s *PowerVSClusterScope) checkTransitGatewayStatus(transitGatewayID *string) error {
	transitGateway, _, err := s.TransitGatewayClient.GetTransitGateway(&tgapiv1.GetTransitGatewayOptions{
		ID: transitGatewayID,
	})
	if err != nil {
		return err
	}
	if transitGateway == nil {
		return fmt.Errorf("tranist gateway is nil")
	}
	if *transitGateway.Status != "available" {
		return fmt.Errorf("error tranist gateway %s not in available status, current status: %s", *transitGatewayID, *transitGateway.Status)
	}

	tgConnections, _, err := s.TransitGatewayClient.ListTransitGatewayConnections(&tgapiv1.ListTransitGatewayConnectionsOptions{
		TransitGatewayID: transitGateway.ID,
	})
	if err != nil {
		return fmt.Errorf("error listing transit gateway connections: %w", err)
	}

	for _, conn := range tgConnections.Connections {
		if *conn.NetworkType == "vpc" && *conn.Status != "attached" {
			return fmt.Errorf("error vpc connection not attached to transit gateway, current status: %s", *conn.Status)
		}
		if *conn.NetworkType == "power_virtual_server" && *conn.Status != "attached" {
			return fmt.Errorf("error powervs connection not attached to transit gateway, current status: %s", *conn.Status)
		}
	}
	return nil
}

// createTransitGateway create transit gateway.
func (s *PowerVSClusterScope) createTransitGateway() (*string, error) {
	// TODO(karthik-k-n): Verify that the supplied zone supports PER

	// TODO(karthik-k-n): consider moving to clusterscope
	// fetch resource group id
	resourceGroupID, err := s.getResourceGroupID()
	if err != nil {
		return nil, fmt.Errorf("error getting id for resource group %s, %w", *s.ResourceGroup(), err)
	}

	transitGatewayName := fmt.Sprintf("%s-%s", s.InfraCluster().GetName(), "transitGateway")
	tg, _, err := s.TransitGatewayClient.CreateTransitGateway(&tgapiv1.CreateTransitGatewayOptions{
		Location:      s.getVPCRegion(),
		Name:          pointer.String(transitGatewayName),
		Global:        pointer.Bool(true),
		ResourceGroup: &tgapiv1.ResourceGroupIdentity{ID: pointer.String(resourceGroupID)},
	})
	if err != nil {
		return nil, fmt.Errorf("error creating transit gateway: %w", err)
	}

	vpcCRN, err := s.fetchVPCCRN()
	if err != nil {
		return nil, fmt.Errorf("error failed to fetch VPC CRN: %w", err)
	}

	_, _, err = s.TransitGatewayClient.CreateTransitGatewayConnection(&tgapiv1.CreateTransitGatewayConnectionOptions{
		TransitGatewayID: tg.ID,
		NetworkType:      pointer.String("vpc"),
		NetworkID:        vpcCRN,
		Name:             pointer.String(fmt.Sprintf("%s-vpc-con", transitGatewayName)),
	})
	if err != nil {
		return nil, fmt.Errorf("error creating vpc connection in transit gateway: %w", err)
	}

	pvsServiceInstanceCRN, err := s.fetchPowerVSServiceInstanceCRN()
	if err != nil {
		return nil, fmt.Errorf("error failed to fetch powervs service instance CRN: %w", err)
	}

	_, _, err = s.TransitGatewayClient.CreateTransitGatewayConnection(&tgapiv1.CreateTransitGatewayConnectionOptions{
		TransitGatewayID: tg.ID,
		NetworkType:      pointer.String("power_virtual_server"),
		NetworkID:        pvsServiceInstanceCRN,
		Name:             pointer.String(fmt.Sprintf("%s-pvs-con", transitGatewayName)),
	})
	if err != nil {
		return nil, fmt.Errorf("error creating powervs connection in transit gateway: %w", err)
	}
	return tg.ID, nil
}

// ReconcileLoadBalancer reconcile loadBalancer.
func (s *PowerVSClusterScope) ReconcileLoadBalancer() error {
	if s.GetLoadBalancerID() != nil {
		loadBalancer, _, err := s.IBMVPCClient.GetLoadBalancer(&vpcv1.GetLoadBalancerOptions{
			ID: s.GetLoadBalancerID(),
		})
		if err != nil {
			return err
		}
		if infrav1beta2.VPCLoadBalancerState(*loadBalancer.ProvisioningStatus) != infrav1beta2.VPCLoadBalancerStateActive {
			return fmt.Errorf("loadbalancer is not in active state, current state %s", *loadBalancer.ProvisioningStatus)
		}
		return nil
	}
	// check VPC subnet exist in cloud
	loadBalancerStatus, err := s.checkLoadBalancer()
	if err != nil {
		return err
	}
	if loadBalancerStatus != nil {
		s.SetLoadBalancerStatus(loadBalancerStatus)
		return nil
	}

	// create loadBalancer
	loadBalancerStatus, err = s.createLoadBalancer()
	if err != nil {
		return err
	}
	s.SetLoadBalancerStatus(loadBalancerStatus)
	return nil
}

// checkLoadBalancer checks loadBalancer in cloud.
func (s *PowerVSClusterScope) checkLoadBalancer() (*infrav1beta2.VPCLoadBalancerStatus, error) {
	loadBalancer, err := s.IBMVPCClient.GetLoadBalancerByName(*s.getServiceName("loadBalancer"))
	if err != nil {
		return nil, err
	}
	if loadBalancer == nil {
		return nil, nil
	}
	state := infrav1beta2.VPCLoadBalancerState(*loadBalancer.ProvisioningStatus)
	return &infrav1beta2.VPCLoadBalancerStatus{
		ID:       loadBalancer.ID,
		State:    &state,
		Hostname: loadBalancer.Hostname,
	}, nil
}

// createLoadBalancer creates loadBalancer.
func (s *PowerVSClusterScope) createLoadBalancer() (*infrav1beta2.VPCLoadBalancerStatus, error) {
	options := &vpcv1.CreateLoadBalancerOptions{}
	loadBalancerName := fmt.Sprintf("%s-%s", s.InfraCluster().GetName(), "loabdlanacer")

	// TODO(karthik-k-n): consider moving resource group id to clusterscope
	// fetch resource group id
	resourceGroupID, err := s.getResourceGroupID()
	if err != nil {
		return nil, fmt.Errorf("error getting id for resource group %s, %w", *s.ResourceGroup(), err)
	}

	options.SetName(loadBalancerName)
	options.SetIsPublic(true)
	options.SetResourceGroup(&vpcv1.ResourceGroupIdentity{
		ID: &resourceGroupID,
	})

	subnetId := s.GetVPCSubnetID()
	if subnetId == nil {
		return nil, fmt.Errorf("error subnet required for load balancer creation")
	}
	subnet := &vpcv1.SubnetIdentity{
		ID: subnetId,
	}
	options.Subnets = append(options.Subnets, subnet)

	options.SetPools([]vpcv1.LoadBalancerPoolPrototype{
		{
			Algorithm:     core.StringPtr("round_robin"),
			HealthMonitor: &vpcv1.LoadBalancerPoolHealthMonitorPrototype{Delay: core.Int64Ptr(5), MaxRetries: core.Int64Ptr(2), Timeout: core.Int64Ptr(2), Type: core.StringPtr("tcp")},
			Name:          core.StringPtr(loadBalancerName + "-pool"),
			Protocol:      core.StringPtr("tcp"),
		},
	})

	options.SetListeners([]vpcv1.LoadBalancerListenerPrototypeLoadBalancerContext{
		{
			Protocol: core.StringPtr("tcp"),
			Port:     core.Int64Ptr(int64(s.APIServerPort())),
			DefaultPool: &vpcv1.LoadBalancerPoolIdentityByName{
				Name: core.StringPtr(loadBalancerName + "-pool"),
			},
		},
	})

	loadBalancer, _, err := s.IBMVPCClient.CreateLoadBalancer(options)
	if err != nil {
		return nil, err
	}
	lbState := infrav1beta2.VPCLoadBalancerState(*loadBalancer.ProvisioningStatus)
	return &infrav1beta2.VPCLoadBalancerStatus{
		ID:       loadBalancer.ID,
		State:    &lbState,
		Hostname: loadBalancer.Hostname,
	}, nil
}

// getResourceGroupID retrieving id of resource group.
func (s *PowerVSClusterScope) getResourceGroupID() (string, error) {
	rmv2, err := resourcemanagerv2.NewResourceManagerV2(&resourcemanagerv2.ResourceManagerV2Options{
		Authenticator: s.session.Options.Authenticator,
	})
	if err != nil {
		return "", err
	}
	if rmv2 == nil {
		return "", fmt.Errorf("unable to get resource controller")
	}
	resourceGroup := s.ResourceGroup()
	rmv2ListResourceGroupOpt := resourcemanagerv2.ListResourceGroupsOptions{Name: resourceGroup, AccountID: &s.session.Options.UserAccount}
	resourceGroupListResult, _, err := rmv2.ListResourceGroups(&rmv2ListResourceGroupOpt)
	if err != nil {
		return "", err
	}

	if resourceGroupListResult != nil && len(resourceGroupListResult.Resources) > 0 {
		rg := resourceGroupListResult.Resources[0]
		resourceGroupID := *rg.ID
		return resourceGroupID, nil
	}

	err = fmt.Errorf("could not retrieve resource group id for %s", *resourceGroup)
	return "", err
}

// getVPCRegion returns region associated with VPC zone.
func (s *PowerVSClusterScope) getVPCRegion() *string {
	region := genUtil.ConstructVPCRegionFromZone(*s.VPCZone())
	return &region
}

// fetchVPCCRN returns VPC CRN
func (s *PowerVSClusterScope) fetchVPCCRN() (*string, error) {
	vpcDetails, _, err := s.IBMVPCClient.GetVPC(&vpcv1.GetVPCOptions{
		ID: s.GetVPCID(),
	})
	if err != nil {
		return nil, err
	}
	return vpcDetails.CRN, nil
}

// fetchPowerVSServiceInstanceCRN returns Power VS service instance CRN.
func (s *PowerVSClusterScope) fetchPowerVSServiceInstanceCRN() (*string, error) {
	serviceInstanceID := s.GetServiceInstanceID()
	pvsDetails, _, err := s.ResourceClient.GetResourceInstance(&resourcecontrollerv2.GetResourceInstanceOptions{
		ID: &serviceInstanceID,
	})
	if err != nil {
		return nil, err
	}
	return pvsDetails.CRN, nil
}

// TODO(karthik-k-n): Decide on proper naming format for services
// getServiceName returns name of given service type from spec or generate a name for it.
func (s *PowerVSClusterScope) getServiceName(serviceType string) *string {
	switch serviceType {
	case "serviceInstance":
		if s.ServiceInstance() == nil || s.ServiceInstance().Name == nil {
			return pointer.String(fmt.Sprintf("%s-serviceInstance", s.InfraCluster().GetName()))
		}
		return s.ServiceInstance().Name
	case "network":
		if s.Network().Name == nil {
			return pointer.String(fmt.Sprintf("%s-network", s.InfraCluster().GetName()))
		}
		return s.ServiceInstance().Name
	case "vpc":
		if s.VPC() == nil || s.VPC().Name == nil {
			return pointer.String(fmt.Sprintf("%s-vpc", s.InfraCluster().GetName()))
		}
		return s.VPC().Name
	case "vpcSubnet":
		if s.VPCSubnet() == nil || s.VPCSubnet().Name == nil {
			return pointer.String(fmt.Sprintf("%s-vpcSubent", s.InfraCluster().GetName()))
		}
		return s.VPCSubnet().Name
	case "transitGateway":
		if s.TransitGateway() != nil || s.TransitGateway().Name != nil {
			return pointer.String(fmt.Sprintf("%s-transitGateway", s.InfraCluster().GetName()))
		}
		return s.TransitGateway().Name
	case "loadBalancer":
		if s.LoadBalancer() != nil || s.LoadBalancer().Name == "" {
			return pointer.String(fmt.Sprintf("%s-loadbalancer", s.InfraCluster().GetName()))
		}
		return &s.LoadBalancer().Name
	case "dhcp":
		return pointer.String(fmt.Sprintf("%s-dhcp", s.InfraCluster().GetName()))
	}
	return nil
}

// DeleteLoadBalancer deletes loadBalancer.
func (s *PowerVSClusterScope) DeleteLoadBalancer() error {
	if !s.deleteResource(LoadBalancer) {
		return nil
	}
	return nil
}

// DeleteVPCSubnet deletes VPC subnet.
func (s *PowerVSClusterScope) DeleteVPCSubnet() error {
	if !s.deleteResource(Subnet) {
		return nil
	}
	return nil
}

// DeleteVPC deletes VPC.
func (s *PowerVSClusterScope) DeleteVPC() error {
	if !s.deleteResource(VPC) {
		return nil
	}
	return nil
}

// DeleteTransitGateway deletes transit gateway.
func (s *PowerVSClusterScope) DeleteTransitGateway() error {
	if !s.deleteResource(TransitGateway) {
		return nil
	}
	return nil
}

// DeleteNetwork deletes network.
func (s *PowerVSClusterScope) DeleteNetwork() error {
	if !s.deleteResource(Network) {
		return nil
	}
	return nil
}

// DeleteServiceInstance deletes service instance.
func (s *PowerVSClusterScope) DeleteServiceInstance() error {
	if !s.deleteResource(ServiceInstance) {
		return nil
	}
	return nil
}

// deleteResource returns true or false to decide on deleting provided resource.
func (s *PowerVSClusterScope) deleteResource(resourceType ResourceType) bool {
	switch resourceType {
	case ServiceInstance:
		serviceInstance := s.ServiceInstance()
		if serviceInstance != nil && (serviceInstance.Name != nil || serviceInstance.ID != nil || serviceInstance.RegEx != nil) {
			return true
		}
	case Network:
		network := s.Network()
		if network.ID != nil || network.Name != nil || network.RegEx != nil {
			return true
		}
	case LoadBalancer:
		loadBalancer := s.LoadBalancer()
		if loadBalancer.Name != "" {
			return true
		}
	case Subnet:
		subnet := s.VPCSubnet()
		if subnet != nil && (subnet.ID != nil || subnet.Name != nil) {
			return true
		}
	case VPC:
		vpc := s.VPC()
		if vpc != nil && (vpc.ID != nil || vpc.Name != nil) {
			return true
		}
	case TransitGateway:
		tg := s.TransitGateway()
		if tg != nil && (tg.ID != nil || tg.Name != nil) {
			return true
		}
	}
	return false
}
