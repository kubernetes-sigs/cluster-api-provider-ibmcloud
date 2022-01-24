/*
Copyright 2022 The Kubernetes Authors.

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

	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/models"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/authenticator"
)

var _ PowerVS = &Service{}

// Service holds the PowerVS Service specific information
type Service struct {
	session        *ibmpisession.IBMPISession
	InstanceClient *instance.IBMPIInstanceClient
	NetworkClient  *instance.IBMPINetworkClient
	ImageClient    *instance.IBMPIImageClient
}

type ServiceOptions struct {
	*ibmpisession.IBMPIOptions

	CloudInstanceID string
}

// CreateInstance creates the virtual machine in the Power VS service instance.
func (s *Service) CreateInstance(body *models.PVMInstanceCreate) (*models.PVMInstanceList, error) {
	return s.InstanceClient.Create(body)
}

// DeleteInstance deletes the virtual machine in the Power VS service instance.
func (s *Service) DeleteInstance(id string) error {
	return s.InstanceClient.Delete(id)
}

// GetAllInstance returns all the virtual machine in the Power VS service instance.
func (s *Service) GetAllInstance() (*models.PVMInstances, error) {
	return s.InstanceClient.GetAll()
}

// GetInstance returns the virtual machine in the Power VS service instance.
func (s *Service) GetInstance(id string) (*models.PVMInstance, error) {
	return s.InstanceClient.Get(id)
}

// GetAllImage returns all the images in the Power VS service instance.
func (s *Service) GetAllImage() (*models.Images, error) {
	return s.ImageClient.GetAll()
}

// GetAllNetwork returns all the networks in the Power VS service instance.
func (s *Service) GetAllNetwork() (*models.Networks, error) {
	return s.NetworkClient.GetAll()
}

// NewService returns a new service for the Power VS api client.
func NewService(options ServiceOptions) (*Service, error) {
	auth, err := authenticator.GetAuthenticator()
	if err != nil {
		return nil, err
	}
	options.Authenticator = auth
	session, err := ibmpisession.NewIBMPISession(options.IBMPIOptions)
	if err != nil {
		return nil, err
	}

	ctx := context.Background()

	return &Service{
		session:        session,
		InstanceClient: instance.NewIBMPIInstanceClient(ctx, session, options.CloudInstanceID),
		NetworkClient:  instance.NewIBMPINetworkClient(ctx, session, options.CloudInstanceID),
		ImageClient:    instance.NewIBMPIImageClient(ctx, session, options.CloudInstanceID),
	}, nil
}
