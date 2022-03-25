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
	"fmt"

	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	"github.com/go-logr/logr"
	"github.com/pkg/errors"
	utils "github.com/ppc64le-cloud/powervs-utils"
	"k8s.io/klog/v2/klogr"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1beta1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/authenticator"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	servicesutils "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
)

const (
	// DEBUGLEVEL indicates the debug level of the logs.
	DEBUGLEVEL = 5
)

// PowerVSClusterScopeParams defines the input parameters used to create a new PowerVSClusterScope.
type PowerVSClusterScopeParams struct {
	Client            client.Client
	Logger            logr.Logger
	Cluster           *capiv1beta1.Cluster
	IBMPowerVSCluster *infrav1beta1.IBMPowerVSCluster
}

// PowerVSClusterScope defines a scope defined around a Power VS Cluster.
type PowerVSClusterScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	IBMPowerVSClient  powervs.PowerVS
	Cluster           *capiv1beta1.Cluster
	IBMPowerVSCluster *infrav1beta1.IBMPowerVSCluster
}

// NewPowerVSClusterScope creates a new PowerVSClusterScope from the supplied parameters.
func NewPowerVSClusterScope(params PowerVSClusterScopeParams) (scope *PowerVSClusterScope, err error) {
	scope = &PowerVSClusterScope{}

	if params.Client == nil {
		err = errors.New("failed to generate new scope from nil Client")
		return
	}
	scope.client = params.Client

	if params.Cluster == nil {
		err = errors.New("failed to generate new scope from nil Cluster")
		return
	}
	scope.Cluster = params.Cluster

	if params.IBMPowerVSCluster == nil {
		err = errors.New("failed to generate new scope from nil IBMPowerVSCluster")
		return
	}
	scope.IBMPowerVSCluster = params.IBMPowerVSCluster

	if params.Logger == (logr.Logger{}) {
		params.Logger = klogr.New()
	}
	scope.Logger = params.Logger

	helper, err := patch.NewHelper(params.IBMPowerVSCluster, params.Client)
	if err != nil {
		err = errors.Wrap(err, "failed to init patch helper")
		return
	}
	scope.patchHelper = helper

	spec := params.IBMPowerVSCluster.Spec

	auth, err := authenticator.GetAuthenticator()
	if err != nil {
		err = errors.Wrap(err, "failed to get authenticator")
		return
	}

	account, err := servicesutils.GetAccount(auth)
	if err != nil {
		err = errors.Wrap(err, "failed to get account")
		return
	}

	rc, err := resourcecontroller.NewService(resourcecontroller.ServiceOptions{})
	if err != nil {
		return
	}

	res, _, err := rc.GetResourceInstance(
		&resourcecontrollerv2.GetResourceInstanceOptions{
			ID: core.StringPtr(spec.ServiceInstanceID),
		})
	if err != nil {
		err = errors.Wrap(err, "failed to get resource instance")
		return
	}

	region, err := utils.GetRegion(*res.RegionID)
	if err != nil {
		err = errors.Wrap(err, "failed to get region")
		return
	}

	options := powervs.ServiceOptions{
		IBMPIOptions: &ibmpisession.IBMPIOptions{
			Debug:       params.Logger.V(DEBUGLEVEL).Enabled(),
			UserAccount: account,
			Region:      region,
			Zone:        *res.RegionID,
		},
		CloudInstanceID: spec.ServiceInstanceID,
	}
	c, err := powervs.NewService(options)
	if err != nil {
		err = fmt.Errorf("failed to create NewIBMPowerVSClient")
		return
	}
	scope.IBMPowerVSClient = c

	return scope, nil
}

// PatchObject persists the cluster configuration and status.
func (s *PowerVSClusterScope) PatchObject() error {
	return s.patchHelper.Patch(context.TODO(), s.IBMPowerVSCluster)
}

// Close closes the current scope persisting the cluster configuration and status.
func (s *PowerVSClusterScope) Close() error {
	return s.PatchObject()
}
