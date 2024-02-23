# Dynamically create infrastructure required for PowerVS cluster

## Motivation
Currently, inorder to create  a PowerVS cluster using cluster api we need to create few resources as prerequisites which includes -
1. Creating a PowerVS workspace
2. Creating a PowerVS Network
3. Creating a port on network

As this involves some prerequisite work which is limiting the true capabilities of cluster api.
Along the similar line today the cluster is accessible to end user via external ip and which is loadbalanced on controlplanes using kube-vip.

## Goal
1. Dynamically create the required cloud resources as a part of cluster creation.
2. Allow users to access the cluster via loadbalancer.

## Proposal

### Cluster API PowerVS cluster components

![powervs-cluster-components.png](../images/powervs-cluster-components.png)

PowerVS workpsace is connected to IBM Cloud VPC with the help of IBM Cloud TransitGateway.

### Proposed API changes

```shell
// IBMPowerVSClusterSpec defines the desired state of IBMPowerVSCluster.
type IBMPowerVSClusterSpec struct {
	// ServiceInstanceID is the id of the power cloud instance where the vsi instance will get deployed.
	// Deprecated: use ServiceInstance instead
	ServiceInstanceID string `json:"serviceInstanceID"`

	// Network is the reference to the Network to use for this cluster.
	// when the field is omitted, A DHCP service will be created in the Power VS server workspace and its private network will be used.
	Network IBMPowerVSResourceReference `json:"network"`

	// DHCPServer is contains the configuration to be used while creating a new DHCP server in PowerVS workspace.
	// when the field is omitted, a default name is constructed and DHCP server will be created.
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
	// when omitted system will dynamically create the service instance
	// +optional
	ServiceInstance *IBMPowerVSResourceReference `json:"serviceInstance,omitempty"`

	// zone is the name of Power VS zone where the cluster will be created
	// possible values can be found here https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-creating-power-virtual-server.
	// +optional
	Zone *string `json:"zone,omitempty"`

	// resourceGroup name under which the resources will be created.
	// when omitted default resource group of the account will be used.
	// +optional
	ResourceGroup *IBMPowerVSResourceReference `json:"resourceGroup,omitempty"`

	// vpc contains information about IBM Cloud VPC resources.
	// +optional
	VPC *VPCResourceReference `json:"vpc,omitempty"`

	// vpcSubnets contains information about IBM Cloud VPC Subnet resources.
	// +optional
	VPCSubnets []Subnet `json:"vpcSubnets,omitempty"`

	// transitGateway contains information about IBM Cloud TransitGateway
	// IBM Cloud TransitGateway helps in establishing network connectivity between IBM Cloud Power VS and VPC infrastructure
	// more information about TransitGateway can be found here https://www.ibm.com/products/transit-gateway.
	// +optional
	TransitGateway *TransitGateway `json:"transitGateway,omitempty"`

	// loadBalancers is optional configuration for configuring loadbalancers to control plane or data plane nodes
	// when specified a vpc loadbalancer will be created and controlPlaneEndpoint will be set with associated hostname of loadbalancer.
	// when omitted user is expected to set controlPlaneEndpoint.
	// +optional
	LoadBalancers []VPCLoadBalancerSpec `json:"loadBalancers,omitempty"`

	// cosInstance contains options to configure a supporting IBM Cloud COS bucket for this
	// cluster - currently used for nodes requiring Ignition
	// (https://coreos.github.io/ignition/) for bootstrapping (requires
	// BootstrapFormatIgnition feature flag to be enabled).
	// +optional
	CosInstance *CosInstance `json:"cosInstance,omitempty"`
}

// IBMPowerVSClusterStatus defines the observed state of IBMPowerVSCluster.
type IBMPowerVSClusterStatus struct {
	// ready is true when the provider resource is ready.
	// +kubebuilder:default=false
	Ready bool `json:"ready"`

	// ResourceGroup is the reference to the Power VS resource group under which the resources will be created.
	ResourceGroup *ResourceReference `json:"resourceGroupID,omitempty"`

	// serviceInstance is the reference to the Power VS service on which the server instance(VM) will be created.
	ServiceInstance *ResourceReference `json:"serviceInstance,omitempty"`

	// networkID is the reference to the Power VS network to use for this cluster.
	Network *ResourceReference `json:"network,omitempty"`

	// dhcpServer is the reference to the Power VS DHCP server.
	DHCPServer *ResourceReference `json:"dhcpServer,omitempty"`

	// vpc is reference to IBM Cloud VPC resources.
	VPC *ResourceReference `json:"vpc,omitempty"`

	// vpcSubnet is reference to IBM Cloud VPC subnet.
	VPCSubnet map[string]ResourceReference `json:"vpcSubnet,omitempty"`

	// transitGateway is reference to IBM Cloud TransitGateway.
	TransitGateway *ResourceReference `json:"transitGateway,omitempty"`

	// cosInstance is reference to IBM Cloud COS Instance resource.
	COSInstance *ResourceReference `json:"cosInstance,omitempty"`

	// loadBalancers reference to IBM Cloud VPC Loadbalancer.
	LoadBalancers map[string]VPCLoadBalancerStatus `json:"loadBalancers,omitempty"`

	// Conditions defines current service state of the IBMPowerVSCluster.
	Conditions capiv1beta1.Conditions `json:"conditions,omitempty"`
}

// DHCPServer contains the DHCP server configurations.
type DHCPServer struct {
	// Optional cidr for DHCP private network
	Cidr *string `json:"cidr,omitempty"`

	// Optional DNS Server for DHCP service
	// +kubebuilder:default="1.1.1.1"
	DNSServer *string `json:"dnsServer,omitempty"`

	// Optional name of DHCP Service. Only alphanumeric characters and dashes are allowed (will be prefixed by DHCP identifier)
	Name *string `json:"name,omitempty"`

	// Optional id of the existing DHCPServer
	ID *string `json:"id,omitempty"`

	// Optional indicates if SNAT will be enabled for DHCP service
	// +kubebuilder:default=true
	Snat *bool `json:"snat,omitempty"`
}

// ResourceReference identifies a resource with id.
type ResourceReference struct {
	// id represents the id of the resource.
	ID *string `json:"id,omitempty"`
	// +kubebuilder:default=false
	// controllerCreated indicates whether the resource is created by the controller.
	ControllerCreated *bool `json:"controllerCreated,omitempty"`
}

// TransitGateway holds the TransitGateway information.
type TransitGateway struct {
	Name *string `json:"name,omitempty"`
	ID   *string `json:"id,omitempty"`
}

// VPCResourceReference is a reference to a specific VPC resource by ID or Name
// Only one of ID or Name may be specified. Specifying more than one will result in
// a validation error.
type VPCResourceReference struct {
	// ID of resource
	// +kubebuilder:validation:MinLength=1
	// +optional
	ID *string `json:"id,omitempty"`

	// Name of resource
	// +kubebuilder:validation:MinLength=1
	// +optional
	Name *string `json:"name,omitempty"`

	// IBM Cloud VPC region
	Region *string `json:"region,omitempty"`
}

// CosInstance represents IBM Cloud COS instance.
type CosInstance struct {
	// Name defines name of IBM cloud COS instance to be created.
	// +kubebuilder:validation:MinLength:=3
	// +kubebuilder:validation:MaxLength:=63
	// +kubebuilder:validation:Pattern=`^[a-z0-9][a-z0-9.-]{1,61}[a-z0-9]$`
	Name string `json:"name,omitempty"`

	// bucketName is IBM cloud COS bucket name
	BucketName string `json:"bucketName,omitempty"`

	// bucketRegion is IBM cloud COS bucket region
	BucketRegion string `json:"bucketRegion,omitempty"`
}

```

### Following resources will be created
1. [PowerVS workspace](https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-creating-power-virtual-server)
2. [PowerVS Network](https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-configuring-subnet) [DHCP service]
3. [VPC](https://cloud.ibm.com/docs/vpc?topic=vpc-about-vpc)
4. [VPC Subnet](https://cloud.ibm.com/docs/vpc?topic=vpc-about-networking-for-vpc)
5. [Transit Gateway](https://www.ibm.com/products/transit-gateway)
6. [VPC Loadbalancer](https://www.ibm.com/products/load-balancer)

### Cluster creation workflow
User is expected to set the annotation ```powervs.cluster.x-k8s.io/create-infra:true``` to IBMPowerVSCluser object to make use of this feature. If not set the cluster creation will proceed with existing way.

User can specify the existing resources in spec, When specified controller will take care of reusing those resources.

When the resource is not set or provided resource with name does not exist in cloud, the controller will create the resource in cloud.

![powervs-cluster-create-workflow.png](../images/powervs-cluster-create-workflow.png)

### Cluster Deletion workflow
The controller will only delete the resources which are created by it.

![powervs-cluster-delete-workflow.png](../images/powervs-cluster-delete-workflow.png)
