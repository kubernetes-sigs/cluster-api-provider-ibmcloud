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

package scope

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/pkg/errors"

	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"
	utils "github.com/ppc64le-cloud/powervs-utils"

	"k8s.io/klog/v2/klogr"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"sigs.k8s.io/cluster-api-provider-ibmcloud/api/v1beta1"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/authenticator"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	servicesutils "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/utils"
)

const BucketAccess = "public"

// PowerVSImageScopeParams defines the input parameters used to create a new PowerVSImageScope.
type PowerVSImageScopeParams struct {
	Client          client.Client
	Logger          logr.Logger
	IBMPowerVSImage *v1beta1.IBMPowerVSImage
}

// PowerVSImageScope defines a scope defined around a Power VS Cluster.
type PowerVSImageScope struct {
	logr.Logger
	client      client.Client
	patchHelper *patch.Helper

	IBMPowerVSClient powervs.PowerVS
	IBMPowerVSImage  *v1beta1.IBMPowerVSImage
}

// NewPowerVSImageScope creates a new PowerVSImageScope from the supplied parameters.
func NewPowerVSImageScope(params PowerVSImageScopeParams) (scope *PowerVSImageScope, err error) {
	scope = &PowerVSImageScope{}

	if params.Client == nil {
		return nil, errors.New("failed to generate new scope from nil Client")
	}
	scope.client = params.Client

	if params.IBMPowerVSImage == nil {
		return nil, errors.New("failed to generate new scope from nil IBMPowerVSImage")
	}
	scope.IBMPowerVSImage = params.IBMPowerVSImage

	if params.Logger == (logr.Logger{}) {
		params.Logger = klogr.New()
	}
	scope.Logger = params.Logger

	helper, err := patch.NewHelper(params.IBMPowerVSImage, params.Client)
	if err != nil {
		return nil, errors.Wrap(err, "failed to init patch helper")
	}

	spec := params.IBMPowerVSImage.Spec

	auth, err := authenticator.GetAuthenticator()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get authenticator")
	}

	account, err := servicesutils.GetAccount(auth)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get account")
	}

	rc, err := resourcecontroller.NewService(resourcecontroller.ServiceOptions{})
	if err != nil {
		return nil, err
	}

	res, _, err := rc.GetResourceInstance(
		&resourcecontrollerv2.GetResourceInstanceOptions{
			ID: core.StringPtr(spec.ServiceInstanceID),
		})
	if err != nil {
		return nil, errors.Wrap(err, "failed to get resource instance")
	}

	region, err := utils.GetRegion(*res.RegionID)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get region")
	}

	options := powervs.ServiceOptions{
		IBMPIOptions: &ibmpisession.IBMPIOptions{
			Debug:       true,
			UserAccount: account,
			Region:      region,
			Zone:        *res.RegionID,
		},
		CloudInstanceID: spec.ServiceInstanceID,
	}
	c, err := powervs.NewService(options)

	if err != nil {
		return nil, fmt.Errorf("failed to create NewIBMPowerVSClient")
	}

	return &PowerVSImageScope{
		Logger:           params.Logger,
		client:           params.Client,
		IBMPowerVSClient: c,
		IBMPowerVSImage:  params.IBMPowerVSImage,
		patchHelper:      helper,
	}, nil
}

func (i *PowerVSImageScope) ensureImageUnique(imageName string) (*models.ImageReference, error) {
	images, err := i.IBMPowerVSClient.GetAllImage()
	if err != nil {
		return nil, err
	}
	for _, img := range images.Images {
		if *img.Name == imageName {
			return img, nil
		}
	}
	return nil, nil
}

// CreateImageCOSBucket creates a power vs image
func (i *PowerVSImageScope) CreateImageCOSBucket() (*models.ImageReference, *models.JobReference, error) {
	s := i.IBMPowerVSImage.Spec
	m := i.IBMPowerVSImage.ObjectMeta

	imageReply, err := i.ensureImageUnique(m.Name)
	if err != nil {
		return nil, nil, err
	} else if imageReply != nil {
		i.Info("Image already exists")
		return imageReply, nil, nil
	}

	lastJob, _ := i.GetImportJob()
	if lastJob != nil {
		if *lastJob.Status.State != "completed" && *lastJob.Status.State != "failed" {
			i.Info("Previous import job not yet fininshed - " + *lastJob.Status.State)
			return nil, nil, nil
		}
	}

	body := &models.CreateCosImageImportJob{
		ImageName:     &m.Name,
		BucketName:    s.Bucket,
		BucketAccess:  core.StringPtr(BucketAccess),
		Region:        s.Region,
		ImageFilename: s.Object,
		StorageType:   s.StorageType,
	}

	jobRef, err := i.IBMPowerVSClient.CreateCosImage(body)
	if err != nil {
		i.Info("Unable to create new import job request")
		return nil, nil, err
	}
	i.Info("New import job request created")
	return nil, jobRef, nil
}

// PatchObject persists the cluster configuration and status.
func (i *PowerVSImageScope) PatchObject() error {
	return i.patchHelper.Patch(context.TODO(), i.IBMPowerVSImage)
}

// Close closes the current scope persisting the cluster configuration and status.
func (i *PowerVSImageScope) Close() error {
	return i.PatchObject()
}

func (i *PowerVSImageScope) DeleteImage() error {
	return i.IBMPowerVSClient.DeleteImage(i.IBMPowerVSImage.Status.ImageID)
}

func (i *PowerVSImageScope) GetImportJob() (*models.Job, error) {
	return i.IBMPowerVSClient.GetCosImages(i.IBMPowerVSImage.Spec.ServiceInstanceID)
}

func (i *PowerVSImageScope) DeleteImportJob() error {
	return i.IBMPowerVSClient.DeleteJob(i.IBMPowerVSImage.Status.JobID)
}

func (i *PowerVSImageScope) SetReady() {
	i.IBMPowerVSImage.Status.Ready = true
}

func (i *PowerVSImageScope) SetNotReady() {
	i.IBMPowerVSImage.Status.Ready = false
}

func (i *PowerVSImageScope) IsReady() bool {
	return i.IBMPowerVSImage.Status.Ready
}

func (i *PowerVSImageScope) SetImageID(id *string) {
	if id != nil {
		i.IBMPowerVSImage.Status.ImageID = *id
	}
}

func (i *PowerVSImageScope) GetImageID() string {
	return i.IBMPowerVSImage.Status.ImageID
}

func (i *PowerVSImageScope) SetImageState(status string) {
	i.IBMPowerVSImage.Status.ImageState = v1beta1.PowerVSImageState(status)
}

func (i *PowerVSImageScope) GetImageState() v1beta1.PowerVSImageState {
	return i.IBMPowerVSImage.Status.ImageState
}

func (i *PowerVSImageScope) SetJobID(id string) {
	i.IBMPowerVSImage.Status.JobID = id
}

func (i *PowerVSImageScope) GetJobID() string {
	return i.IBMPowerVSImage.Status.JobID
}
