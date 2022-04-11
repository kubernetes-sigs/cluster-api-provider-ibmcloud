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
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// IBMVPCClients hosts the IBM VPC service.
type IBMVPCClients struct {
	VPCService *vpcv1.VpcV1
	// APIKey          string
	// IAMEndpoint     string
	// ServiceEndPoint string
}

func (c *IBMVPCClients) setIBMVPCService(authenticator core.Authenticator, svcEndpoint string) error {
	var err error
	c.VPCService, err = vpcv1.NewVpcV1(&vpcv1.VpcV1Options{
		Authenticator: authenticator,
		URL:           svcEndpoint,
	})

	return err
}
