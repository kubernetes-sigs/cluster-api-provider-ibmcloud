package containerservice

import (
	"fmt"

	"github.com/IBM/container-services-go-sdk/kubernetesserviceapiv1"
	"github.com/IBM/go-sdk-core/v5/core"
)

// Client wraps the IBM Cloud Kubernetes Service API client
type Client struct {
	service *kubernetesserviceapiv1.KubernetesServiceApiV1
	apiKey  string
	region  string
}

// NewClient creates a new Container Service API client using the IBM Cloud SDK
func NewClient(apiKey, region string) (*Client, error) {
	// Create authenticator
	authenticator := &core.IamAuthenticator{
		ApiKey: apiKey,
	}

	// Create service
	service, err := kubernetesserviceapiv1.NewKubernetesServiceApiV1(&kubernetesserviceapiv1.KubernetesServiceApiV1Options{
		Authenticator: authenticator,
		URL:           "https://containers.cloud.ibm.com/global",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create service: %w", err)
	}

	return &Client{
		service: service,
		apiKey:  apiKey,
		region:  region,
	}, nil
}
