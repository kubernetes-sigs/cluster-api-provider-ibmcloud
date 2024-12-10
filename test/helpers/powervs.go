/*
Copyright 2024 The Kubernetes Authors.

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

package helpers

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/IBM-Cloud/power-go-client/clients/instance"
	"github.com/IBM-Cloud/power-go-client/ibmpisession"

	"k8s.io/apimachinery/pkg/util/wait"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/authenticator"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
)

var (
	pollingInterval                = time.Second * 30
	powerVSInstanceDeletionTimeout = time.Minute * 10
)

const (
	powervsInstanceDoesNotExist  = "pvm-instance does not exist"
	powervsInstanceNotFound      = "could not be found"
	powervsInstanceStateDeleting = "deleting"
)

// VerifyServerInstancesDeletion checks if the virtual server instances
// are deleted in a given PowerVS workspace.
func VerifyServerInstancesDeletion(serviceInstanceID string) error {
	pclient, err := getPowerVSInstanceClient(serviceInstanceID)
	if err != nil {
		return err
	}

	instances, err := pclient.GetAll()
	if err != nil {
		return err
	}

	for _, ins := range instances.PvmInstances {
		err = wait.PollUntilContextTimeout(context.Background(), pollingInterval, powerVSInstanceDeletionTimeout, false, func(_ context.Context) (done bool, err error) {
			instance, err := pclient.Get(*ins.PvmInstanceID)
			if err != nil {
				if strings.Contains(err.Error(), powervsInstanceNotFound) || strings.Contains(err.Error(), powervsInstanceDoesNotExist) {
					return true, nil
				}
				return false, err
			}

			if instance.TaskState == powervsInstanceStateDeleting {
				return false, nil
			}
			return false, nil
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func getPowerVSInstanceClient(serviceInstanceID string) (*instance.IBMPIInstanceClient, error) {
	auth, err := authenticator.GetAuthenticator()
	if err != nil {
		return nil, err
	}

	zone := os.Getenv("IBMPOWERVS_ZONE")
	if zone == "" {
		return nil, fmt.Errorf("IBMPOWERVS_ZONE is not set")
	}

	account, err := utils.GetAccount(auth)
	if err != nil {
		return nil, err
	}

	piOptions := ibmpisession.IBMPIOptions{
		Authenticator: auth,
		UserAccount:   account,
		Zone:          zone,
		Debug:         true,
	}

	session, err := ibmpisession.NewIBMPISession(&piOptions)
	if err != nil {
		return nil, err
	}
	return instance.NewIBMPIInstanceClient(context.Background(), session, serviceInstanceID), nil
}
