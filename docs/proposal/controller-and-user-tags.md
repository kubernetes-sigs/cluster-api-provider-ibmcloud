# Add tags to PowerVS Cluster resources and delete on the bases of specific tag


## Motivation
PowerVS cluster reconciler sets controllercreated field whenever resource is created by controller, which was initially introduced to allow proper cleanup of newly created resource vs the use of existing resources.

Though its working as expected and fulfills the purpose, we see some drawbacks.
1. The field is initially set to true during the first reconciliation cycle when the resource is being created. In subsequent reconciliation loops, the field is not updated because the resource already exists in the cloud. This behavior introduces non-idempotency in the controller logic. As a result, if the initial reconciliation event is missed, the controller exhibits inconsistent behavior. Its against k8s principle of reconcilation of having level trigger rather than edge triggered.
2. The status is expected to be created from spec, considering the scenario of backup and recover. If we move the spec to fresh management cluster which is setting the status, the controller created will be set as false as the resource already exists in cloud but it was created during its previous concilation.

## Goal
1. This proposal aims to tag the PowerVS clusters and delete the resources created by controller based on tag.
2. User should be able to add tags to the resources if he wants.

## Proposal
This proposal presents adding two kinds of tags to the resources created by controller
1. Controller tag
2. User tags


### Controller tag
Add Controller tag `powervs.cluster.x-k8s.io-resource-owner:<cluster_name>` to handle deletion of PowerVS Cluster resources created by controller


#### Following resources will be getting tagged 
1. [PowerVS workspace](https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-creating-power-virtual-server)
2. [PowerVS Network](https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-configuring-subnet) [DHCP service]
3. [VPC](https://cloud.ibm.com/docs/vpc?topic=vpc-about-vpc)
4. [VPC Subnet](https://cloud.ibm.com/docs/vpc?topic=vpc-about-networking-for-vpc)
5. [VPC Security Groups](https://cloud.ibm.com/docs/vpc?topic=vpc-security-in-your-vpc)
6. [Transit Gateway](https://www.ibm.com/products/transit-gateway)
7. [VPC Loadbalancer](https://www.ibm.com/products/load-balancer)

#### Note 
- Currently TransitGateway Connections doesn't support tagging, So we will handle deletion of connections based on VPC.
- DHCP Server doesn't support tagging, So we will tag DHCP Network and handle deletion based on Network.


### User tags
User can add tags to resources when creating PowerVS cluster.

#### Proposed API changes
UserTags field will contain list of tags that will be applied on resources.

```shell

// IBMPowerVSClusterSpec defines the desired state of IBMPowerVSCluster.
type IBMPowerVSClusterSpec struct {

	// UserTags contains list of tags needs to be applied on resources
	UserTags []string `json:"tags,omitempty"`

	// ServiceInstanceID is the id of the power cloud instance where the vsi instance will get deployed.
	// Deprecated: use ServiceInstance instead
	ServiceInstanceID string `json:"serviceInstanceID"`

	// Network is the reference to the Network to use for this cluster.
	// when the field is omitted, A DHCP service will be created in the Power VS workspace and its private network will be used.
	// the DHCP service created network will have the following name format
	// 1. in the case of DHCPServer.Name is not set the name will be DHCPSERVER<CLUSTER_NAME>_Private.
	// 2. if DHCPServer.Name is set the name will be DHCPSERVER<DHCPServer.Name>_Private.
	// when Network.ID is set, its expected that there exist a network in PowerVS workspace with id or else system will give error.
	// when Network.Name is set, system will first check for network with Name in PowerVS workspace, if not exist system will check DHCP network with given Network.name, if that also not exist, it will create a new DHCP service and name will be DHCPSERVER<Network.Name>_Private.
	// Network.RegEx is not yet supported and system will ignore the value.
	Network IBMPowerVSResourceReference `json:"network"`

	// dhcpServer is contains the configuration to be used while creating a new DHCP server in PowerVS workspace.
	// when the field is omitted, CLUSTER_NAME will be used as DHCPServer.Name and DHCP server will be created.
	// it will automatically create network with name DHCPSERVER<DHCPServer.Name>_Private in PowerVS workspace.
	// +optional
	DHCPServer *DHCPServer `json:"dhcpServer,omitempty"`

	// ControlPlaneEndpoint represents the endpoint used to communicate with the control plane.
	// +optional
	ControlPlaneEndpoint capiv1beta1.APIEndpoint `json:"controlPlaneEndpoint"`

	// serviceInstance is the reference to the Power VS server workspace on which the server instance(VM) will be created.
	// Power VS server workspace is a container for all Power VS instances at a specific geographic region.
	// serviceInstance can be created via IBM Cloud catalog or CLI.
	// supported serviceInstance identifier in PowerVSResource are Name and ID and that can be obtained from IBM Cloud UI or IBM Cloud cli.
	// More detail about Power VS service instance.
	// https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-creating-power-virtual-server
	// when omitted system will dynamically create the service instance with name CLUSTER_NAME-serviceInstance.
	// when ServiceInstance.ID is set, its expected that there exist a service instance in PowerVS workspace with id or else system will give error.
	// when ServiceInstance.Name is set, system will first check for service instance with Name in PowerVS workspace, if not exist system will create new instance.
	// if there are more than one service instance exist with the ServiceInstance.Name in given Zone, installation fails with an error. Use ServiceInstance.ID in those situations to use the specific service instance.
	// ServiceInstance.Regex is not yet supported not yet supported and system will ignore the value.
	// +optional
	ServiceInstance *IBMPowerVSResourceReference `json:"serviceInstance,omitempty"`

	// zone is the name of Power VS zone where the cluster will be created
	// possible values can be found here https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-creating-power-virtual-server.
	// when powervs.cluster.x-k8s.io/create-infra=true annotation is set on IBMPowerVSCluster resource,
	// 1. it is expected to set the zone, not setting will result in webhook error.
	// 2. the zone should have PER capabilities, or else system will give error.
	// +optional
	Zone *string `json:"zone,omitempty"`

	// resourceGroup name under which the resources will be created.
	// when powervs.cluster.x-k8s.io/create-infra=true annotation is set on IBMPowerVSCluster resource,
	// 1. it is expected to set the ResourceGroup.Name, not setting will result in webhook error.
	// ResourceGroup.ID and ResourceGroup.Regex is not yet supported and system will ignore the value.
	// +optional
	ResourceGroup *IBMPowerVSResourceReference `json:"resourceGroup,omitempty"`

	// vpc contains information about IBM Cloud VPC resources.
	// when omitted system will dynamically create the VPC with name CLUSTER_NAME-vpc.
	// when VPC.ID is set, its expected that there exist a VPC with ID or else system will give error.
	// when VPC.Name is set, system will first check for VPC with Name, if not exist system will create new VPC.
	// when powervs.cluster.x-k8s.io/create-infra=true annotation is set on IBMPowerVSCluster resource,
	// 1. it is expected to set the VPC.Region, not setting will result in webhook error.
	// +optional
	VPC *VPCResourceReference `json:"vpc,omitempty"`

	// vpcSubnets contains information about IBM Cloud VPC Subnet resources.
	// when omitted system will create the subnets in all the zone corresponding to VPC.Region, with name CLUSTER_NAME-vpcsubnet-ZONE_NAME.
	// possible values can be found here https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-creating-power-virtual-server.
	// when VPCSubnets[].ID is set, its expected that there exist a subnet with ID or else system will give error.
	// when VPCSubnets[].Zone is not set, a random zone is picked from available zones of VPC.Region.
	// when VPCSubnets[].Name is not set, system will set name as CLUSTER_NAME-vpcsubnet-INDEX.
	// if subnet with name VPCSubnets[].Name not found, system will create new subnet in VPCSubnets[].Zone.
	// +optional
	VPCSubnets []Subnet `json:"vpcSubnets,omitempty"`

	// VPCSecurityGroups to attach it to the VPC resource
	// +optional
	VPCSecurityGroups []VPCSecurityGroup `json:"vpcSecurityGroups,omitempty"`

	// transitGateway contains information about IBM Cloud TransitGateway
	// IBM Cloud TransitGateway helps in establishing network connectivity between IBM Cloud Power VS and VPC infrastructure
	// more information about TransitGateway can be found here https://www.ibm.com/products/transit-gateway.
	// when TransitGateway.ID is set, its expected that there exist a TransitGateway with ID or else system will give error.
	// when TransitGateway.Name is set, system will first check for TransitGateway with Name, if not exist system will create new TransitGateway.
	// +optional
	TransitGateway *TransitGateway `json:"transitGateway,omitempty"`

	// loadBalancers is optional configuration for configuring loadbalancers to control plane or data plane nodes.
	// when omitted system will create a default public loadbalancer with name CLUSTER_NAME-loadbalancer.
	// when specified a vpc loadbalancer will be created and controlPlaneEndpoint will be set with associated hostname of loadbalancer.
	// ControlPlaneEndpoint will be set with associated hostname of public loadbalancer.
	// when LoadBalancers[].ID is set, its expected that there exist a loadbalancer with ID or else system will give error.
	// when LoadBalancers[].Name is set, system will first check for loadbalancer with Name, if not exist system will create new loadbalancer.
	// For each loadbalancer a default backed pool and front listener will be configured with port 6443.
	// +optional
	LoadBalancers []VPCLoadBalancerSpec `json:"loadBalancers,omitempty"`

	// cosInstance contains options to configure a supporting IBM Cloud COS bucket for this
	// cluster - currently used for nodes requiring Ignition
	// (https://coreos.github.io/ignition/) for bootstrapping (requires
	// BootstrapFormatIgnition feature flag to be enabled).
	// when powervs.cluster.x-k8s.io/create-infra=true annotation is set on IBMPowerVSCluster resource and Ignition is set, then
	// 1. CosInstance.Name should be set not setting will result in webhook error.
	// 2. CosInstance.BucketName should be set not setting will result in webhook error.
	// 3. CosInstance.BucketRegion should be set not setting will result in webhook error.
	// +optional
	CosInstance *CosInstance `json:"cosInstance,omitempty"`

	// Ignition defined options related to the bootstrapping systems where Ignition is used.
	// +optional
	Ignition *Ignition `json:"ignition,omitempty"`
}

```


### Cluster creation workflow
The controller will attach the tag to the resources after resources are created.
![add-tag-workflow.png](../images/add-tag-workflow.png)


### Cluster Deletion workflow
The controller will only delete the resources which are having this tag `powervs.cluster.x-k8s.io-resource-owner:<cluster_name>` attched to it.
![delete-tag-workflow.png](../images/delete-tag-workflow.png)