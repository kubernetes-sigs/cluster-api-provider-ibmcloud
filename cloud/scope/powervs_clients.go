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
	"time"

	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"
)

// TIMEOUT is used while creating IBM Power VS client
const TIMEOUT = 1 * time.Hour

// IBMPowerVSClient used to store IBM Power VS client information
type IBMPowerVSClient struct {
	session        *ibmpisession.IBMPISession
	InstanceClient *instance.IBMPIInstanceClient
	NetworkClient  *instance.IBMPINetworkClient
}

// NewIBMPowerVSClient creates and returns a IBM Power VS client
func NewIBMPowerVSClient(token, account, cloudInstanceID, region, zone string, debug bool) (_ *IBMPowerVSClient, err error) {
	client := &IBMPowerVSClient{}
	client.session, err = ibmpisession.New(token, region, debug, TIMEOUT, account, zone)
	if err != nil {
		return nil, err
	}

	client.InstanceClient = instance.NewIBMPIInstanceClient(client.session, cloudInstanceID)
	client.NetworkClient = instance.NewIBMPINetworkClient(client.session, cloudInstanceID)
	return client, nil
}
