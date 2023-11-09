/*
Copyright 2023 The Kubernetes Authors.

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

package transitgateway

import (
	"fmt"
	"github.com/IBM/go-sdk-core/v5/core"
	"time"

	tgapiv1 "github.com/IBM/networking-go-sdk/transitgatewayapisv1"

	"k8s.io/utils/pointer"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/authenticator"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
)

var currentDate = fmt.Sprintf("%d-%02d-%02d", time.Now().Year(), time.Now().Month(), time.Now().Day())

// Service holds the IBM Cloud Resource Controller Service specific information.
type Service struct {
	tgClient *tgapiv1.TransitGatewayApisV1
}

// NewService returns a new service for the IBM Cloud Transit Gateway api client.
func NewService() (TransitGateway, error) {
	auth, err := authenticator.GetAuthenticator()
	if err != nil {
		return nil, err
	}
	tgClient, err := tgapiv1.NewTransitGatewayApisV1(&tgapiv1.TransitGatewayApisV1Options{
		Authenticator: auth,
		Version:       pointer.String(currentDate),
	})

	return &Service{
		tgClient: tgClient,
	}, nil
}

func (s *Service) GetTransitGateway(options *tgapiv1.GetTransitGatewayOptions) (*tgapiv1.TransitGateway, *core.DetailedResponse, error) {
	return s.tgClient.GetTransitGateway(options)
}

func (s *Service) GetTransitGatewayByName(name string) (*tgapiv1.TransitGateway, error) {
	var transitGateway *tgapiv1.TransitGateway

	f := func(start string) (bool, string, error) {
		tgList, _, err := s.tgClient.ListTransitGateways(&tgapiv1.ListTransitGatewaysOptions{})
		if err != nil {
			return false, "", fmt.Errorf("failed to list transit gateway %w", err)
		}

		for _, tg := range tgList.TransitGateways {
			if *tg.Name == name {
				transitGateway = &tg
				return true, "", nil
			}
		}

		if tgList.Next != nil && *tgList.Next.Href != "" {
			return false, *tgList.Next.Href, nil
		}

		return true, "", nil
	}

	if err := utils.PagingHelper(f); err != nil {
		return nil, err
	}
	return transitGateway, nil
}

func (s *Service) ListTransitGatewayConnections(options *tgapiv1.ListTransitGatewayConnectionsOptions) (*tgapiv1.TransitGatewayConnectionCollection, *core.DetailedResponse, error) {
	return s.tgClient.ListTransitGatewayConnections(options)
}

func (s *Service) CreateTransitGateway(options *tgapiv1.CreateTransitGatewayOptions) (*tgapiv1.TransitGateway, *core.DetailedResponse, error) {
	return s.tgClient.CreateTransitGateway(options)
}

func (s *Service) CreateTransitGatewayConnection(options *tgapiv1.CreateTransitGatewayConnectionOptions) (*tgapiv1.TransitGatewayConnectionCust, *core.DetailedResponse, error) {
	return s.tgClient.CreateTransitGatewayConnection(options)
}

func (s *Service) GetTransitGatewayConnection(options *tgapiv1.GetTransitGatewayConnectionOptions) (*tgapiv1.TransitGatewayConnectionCust, *core.DetailedResponse, error) {
	return s.tgClient.GetTransitGatewayConnection(options)
}
