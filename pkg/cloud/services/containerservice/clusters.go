package containerservice

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/IBM/container-services-go-sdk/kubernetesserviceapiv1"
	"github.com/IBM/go-sdk-core/v5/core"
)

// Helper functions for pointer dereferencing
func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func int64Value(i *int64) int64 {
	if i == nil {
		return 0
	}
	return *i
}

// CreateClusterRequest represents a request to create a ROKS cluster
type CreateClusterRequest struct {
	Name                         string
	MasterVersion                string
	MachineType                  string
	WorkerNum                    int
	PodSubnet                    string
	ServiceSubnet                string
	PrivateServiceEndpoint       bool
	PublicServiceEndpoint        bool
	DiskEncryption               bool
	DefaultWorkerPoolName        string
	DefaultWorkerPoolEntitlement string
	CosInstanceCRN               string
	// VPC-specific fields
	VPCID string
	Zones []ClusterZone
}

// ClusterZone represents a zone configuration for cluster creation
type ClusterZone struct {
	ID       string
	SubnetID string
}

// ClusterResponse represents a cluster from the API
type ClusterResponse struct {
	ID                string
	Name              string
	Region            string
	DataCenter        string
	State             string
	MasterStatus      string
	MasterURL         string
	IngressHostname   string
	IngressSecretName string
	WorkerCount       int
	MasterKubeVersion string
	CreatedDate       string
	ResourceGroupID   string
	Provider          string
	Type              string
	Entitlement       string
}

// CreateCluster creates a new ROKS cluster using the IBM Cloud SDK
// For VPC clusters, uses VpcCreateCluster; for Classic, uses CreateCluster
func (c *Client) CreateCluster(ctx context.Context, req *CreateClusterRequest, resourceGroupID string) (*ClusterResponse, error) {
	// Debug: log the request
	fmt.Printf("DEBUG: CreateCluster called with VPCID=%s, Zones=%+v\n", req.VPCID, req.Zones)

	// For VPC clusters (which is what we're targeting for ROKS), use VpcCreateCluster
	// Build VPC create options using the SDK
	options := c.service.NewVpcCreateClusterOptions()

	// Set required fields
	options.SetName(req.Name)
	options.SetKubeVersion(req.MasterVersion)
	options.SetProvider("vpc-gen2") // VPC Gen 2

	// Build worker pool with VPC-specific configuration
	// IMPORTANT: VpcID must be set here in the worker pool, not at cluster level
	workerPool := &kubernetesserviceapiv1.VPCCreateClusterWorkerPool{
		Flavor:      &req.MachineType,
		Name:        &req.DefaultWorkerPoolName,
		VpcID:       &req.VPCID, // This is where VPC ID goes for VPC clusters
		WorkerCount: core.Int64Ptr(int64(req.WorkerNum)),
	}

	// Add disk encryption if specified
	if req.DiskEncryption {
		workerPool.DiskEncryption = core.BoolPtr(true)
	}

	// Convert zones with subnet IDs
	if len(req.Zones) > 0 {
		zones := make([]kubernetesserviceapiv1.VPCCreateClusterWorkerPoolZone, len(req.Zones))
		for i, z := range req.Zones {
			zones[i] = kubernetesserviceapiv1.VPCCreateClusterWorkerPoolZone{
				ID:       &z.ID,
				SubnetID: &z.SubnetID,
			}
		}
		workerPool.Zones = zones
	}

	options.SetWorkerPool(workerPool)

	// Set optional fields
	if req.PodSubnet != "" {
		options.SetPodSubnet(req.PodSubnet)
	}
	if req.ServiceSubnet != "" {
		options.SetServiceSubnet(req.ServiceSubnet)
	}
	if !req.PublicServiceEndpoint {
		// Disable public endpoint if not requested
		options.SetDisablePublicServiceEndpoint(true)
	}
	if req.DefaultWorkerPoolEntitlement != "" {
		options.SetDefaultWorkerPoolEntitlement(req.DefaultWorkerPoolEntitlement)
	}
	if req.CosInstanceCRN != "" {
		options.SetCosInstanceCRN(req.CosInstanceCRN)
	}

	// Set resource group header
	if resourceGroupID != "" {
		options.SetXAuthResourceGroup(resourceGroupID)
	}

	// Create the cluster - this only returns the cluster ID
	result, response, err := c.service.VpcCreateClusterWithContext(ctx, options)
	if err != nil {
		if response != nil {
			// Try to extract detailed error message from response body
			bodyBytes := []byte{}
			if response.Result != nil {
				bodyBytes, _ = json.Marshal(response.Result)
			}
			return nil, fmt.Errorf("API error (status %d): %v, response: %s", response.StatusCode, err, string(bodyBytes))
		}
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}

	// The create response only contains the ClusterID, so we need to fetch full details
	if result.ClusterID == nil {
		return nil, fmt.Errorf("cluster creation succeeded but no ID returned")
	}

	// Get full cluster details
	return c.GetCluster(ctx, *result.ClusterID, resourceGroupID)
}

// GetCluster retrieves cluster information using the IBM Cloud SDK
func (c *Client) GetCluster(ctx context.Context, clusterID, resourceGroupID string) (*ClusterResponse, error) {
	options := c.service.NewGetClusterOptions(clusterID)

	if resourceGroupID != "" {
		options.SetXAuthResourceGroup(resourceGroupID)
	}

	result, response, err := c.service.GetClusterWithContext(ctx, options)
	if err != nil {
		if response != nil && response.StatusCode == 404 {
			return nil, fmt.Errorf("cluster not found: %s", clusterID)
		}
		if response != nil {
			return nil, fmt.Errorf("API error (status %d): %v", response.StatusCode, err)
		}
		return nil, fmt.Errorf("failed to get cluster: %w", err)
	}

	// Convert SDK response to our response type
	// Map available fields from GetClusterResponse
	cluster := &ClusterResponse{
		ID:                stringValue(result.ID),
		Name:              stringValue(result.Name),
		Region:            stringValue(result.Region),
		DataCenter:        stringValue(result.Datacenter),
		State:             stringValue(result.State),
		MasterStatus:      stringValue(result.Status), // Status field, not MasterStatus
		MasterURL:         stringValue(result.MasterURL),
		WorkerCount:       int(int64Value(result.WorkerCount)),
		MasterKubeVersion: stringValue(result.MasterKubeVersion),
		ResourceGroupID:   stringValue(result.ResourceGroup),
		Provider:          stringValue(result.Provider),
		Type:              stringValue(result.Type),
		CreatedDate:       stringValue(result.CreatedDate),
	}

	// Extract ingress information if available
	if result.Ingress != nil {
		cluster.IngressHostname = stringValue(result.Ingress.Hostname)
		cluster.IngressSecretName = stringValue(result.Ingress.SecretName)
	}

	return cluster, nil
}

// DeleteCluster deletes a cluster using the IBM Cloud SDK
func (c *Client) DeleteCluster(ctx context.Context, clusterID, resourceGroupID string) error {
	options := c.service.NewRemoveClusterOptions(clusterID)

	if resourceGroupID != "" {
		options.SetXAuthResourceGroup(resourceGroupID)
	}

	response, err := c.service.RemoveClusterWithContext(ctx, options)
	if err != nil {
		if response != nil {
			return fmt.Errorf("API error (status %d): %v", response.StatusCode, err)
		}
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	return nil
}
