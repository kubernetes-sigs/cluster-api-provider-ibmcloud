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
	"errors"
	"fmt"

	"github.com/IBM-Cloud/power-go-client/ibmpisession"
	"github.com/IBM-Cloud/power-go-client/power/models"
	"github.com/IBM/platform-services-go-sdk/resourcecontrollerv2"

	"k8s.io/utils/ptr"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	infrav1 "sigs.k8s.io/cluster-api-provider-ibmcloud/api/powervs/v1beta3"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/powervs"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/services/resourcecontroller"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/endpoints"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/record"
)

// BucketAccess indicates if the bucket has public or private access public access.
const BucketAccess = "public"

var (
	// ErrWorkspaceNotInActiveState indicates error if workspace is inactive.
	ErrWorkspaceNotInActiveState = errors.New("workspace is not in active state")
)

// ImageScopeParams defines the input parameters used to create a new ImageScope.
type ImageScopeParams struct {
	Client          client.Client
	IBMPowerVSImage *infrav1.IBMPowerVSImage
	ServiceEndpoint []endpoints.ServiceEndpoint
	Zone            *string
}

// ImageScope defines a scope defined around a Power VS Cluster.
type ImageScope struct {
	Client           client.Client
	IBMPowerVSClient powervs.PowerVS
	IBMPowerVSImage  *infrav1.IBMPowerVSImage
	ServiceEndpoint  []endpoints.ServiceEndpoint
	workspaceID      string
}

// NewPowerVSImageScope creates a new ImageScope from the supplied parameters.
func NewPowerVSImageScope(ctx context.Context, params ImageScopeParams) (scope *ImageScope, err error) {
	log := ctrl.LoggerFrom(ctx)
	scope = &ImageScope{}

	if params.Client == nil {
		err = errors.New("failed to generate new scope from nil Client")
		return nil, err
	}
	scope.Client = params.Client

	if params.IBMPowerVSImage == nil {
		err = errors.New("failed to generate new scope from nil IBMPowerVSImage")
		return nil, err
	}
	scope.IBMPowerVSImage = params.IBMPowerVSImage

	// Create Resource Controller client.
	var serviceOption resourcecontroller.ServiceOptions
	// Fetch the resource controller endpoint.
	rcEndpoint := endpoints.FetchEndpoints(string(endpoints.RC), params.ServiceEndpoint)
	if rcEndpoint != "" {
		serviceOption.URL = rcEndpoint
		log.V(3).Info("Overriding the default resource controller endpoint", "ResourceControllerEndpoint", rcEndpoint)
	}

	rc, err := resourcecontroller.NewService(serviceOption)
	if err != nil {
		return nil, err
	}

	var workspaceID, workspaceName string

	// 1. Fast Path: The user provided the exact ID in the Image Spec.
	if params.IBMPowerVSImage.Spec.Workspace.ID != "" {
		workspaceID = params.IBMPowerVSImage.Spec.Workspace.ID
	} else {
		// 2. Lookup Path: We need to find the ID using a Name.
		if params.IBMPowerVSImage.Spec.Workspace.Name != "" {
			workspaceName = params.IBMPowerVSImage.Spec.Workspace.Name
		} else {
			// The user provided nothing. Infer the default name using the ClusterName.
			workspaceName = fmt.Sprintf("%s-workspace", params.IBMPowerVSImage.Spec.ClusterName)
		}

		resourceInstance := resourcecontroller.InstanceFilter{
			Name:           workspaceName,
			Zone:           params.Zone,
			ResourceID:     resourcecontroller.PowerVSResourceID,
			ResourcePlanID: resourcecontroller.PowerVSResourcePlanID,
		}

		workspace, err := rc.GetResourceInstanceByFilter(resourceInstance)
		if err != nil {
			return nil, fmt.Errorf("failed to look up workspace name %q: %w", workspaceName, err)
		}

		if workspace == nil || workspace.GUID == nil {
			return nil, fmt.Errorf("PowerVS workspace %q is not yet created or GUID is nil", workspaceName)
		}

		if workspace.State == nil || *workspace.State != string(infrav1.WorkspaceStateActive) {
			return scope, fmt.Errorf("PowerVS workspace (name: %q) is not in active state", workspaceName)
		}

		workspaceID = *workspace.GUID
	}

	res, _, err := rc.GetResourceInstance(
		&resourcecontrollerv2.GetResourceInstanceOptions{
			ID: &workspaceID,
		})
	if err != nil {
		err = fmt.Errorf("failed to get resource instance: %w", err)
		return nil, err
	}

	options := powervs.ServiceOptions{
		IBMPIOptions: &ibmpisession.IBMPIOptions{
			Debug: log.V(DEBUGLEVEL).Enabled(),
			Zone:  *res.RegionID,
		},
	}

	// Fetch the service endpoint.
	if svcEndpoint := endpoints.FetchPVSEndpoint(endpoints.ConstructRegionFromZone(*res.RegionID), params.ServiceEndpoint); svcEndpoint != "" {
		options.IBMPIOptions.URL = svcEndpoint
		log.V(3).Info("Overriding the default PowerVS service endpoint", "serviceEndpoint", svcEndpoint)
	}

	c, err := powervs.NewService(options)
	if err != nil {
		err = fmt.Errorf("failed to create NewIBMPowerVSClient error %w", err)
		return nil, err
	}

	options.CloudInstanceID = workspaceID
	c.WithClients(options)
	scope.IBMPowerVSClient = c
	scope.workspaceID = workspaceID
	return scope, nil
}

// GetOrImportImage verifies if the image exists, and if not, triggers a COS import job.
func (i *ImageScope) GetOrImportImage(ctx context.Context) (*models.ImageReference, *models.JobReference, error) {
	log := ctrl.LoggerFrom(ctx)
	imageSpec := i.IBMPowerVSImage.Spec
	imageName := i.IBMPowerVSImage.Name

	// 1. Idempotency Check
	imageReply, err := i.ensureImageUnique(imageName)
	if err != nil {
		record.Warnf(i.IBMPowerVSImage, "FailedRetrieveImage", "Failed to retrieve image %q", imageName)
		return nil, nil, fmt.Errorf("failed to verify image uniqueness: %w", err)
	} else if imageReply != nil {
		log.Info("Image already exists", "imageName", imageName)
		return imageReply, nil, nil
	}

	// 2. In-Progress Job Check
	if lastJob, _ := i.getImportJob(); lastJob != nil && lastJob.Status != nil && lastJob.Status.State != nil {
		state := *lastJob.Status.State
		if state != string(infrav1.PowerVSImageStateCompleted) && state != string(infrav1.PowerVSImageStateFailed) {
			log.Info("Previous import job not yet finished", "state", state)
			return nil, nil, nil
		}
	}

	// 3. Trigger New Import Job
	body := &models.CreateCosImageImportJob{
		ImageName:     ptr.To(imageName),
		BucketName:    ptr.To(imageSpec.Bucket),
		BucketAccess:  ptr.To(BucketAccess),
		Region:        ptr.To(imageSpec.Region),
		ImageFilename: ptr.To(imageSpec.Object),
	}

	if imageSpec.StorageType != "" {
		body.StorageType = imageSpec.StorageType
	}

	jobRef, err := i.IBMPowerVSClient.CreateCosImage(body)
	if err != nil {
		record.Warnf(i.IBMPowerVSImage, "FailedCreateImageImportJob", "Failed image import job creation: %v", err)
		return nil, nil, fmt.Errorf("failed to create COS image import job: %w", err)
	}

	log.Info("New import job request created", "jobID", *jobRef.ID)
	record.Eventf(i.IBMPowerVSImage, "SuccessfulCreateImageImportJob", "Created image import job %q", *jobRef.ID)
	return nil, jobRef, nil
}

// DeleteImage will delete the image.
func (i *ImageScope) DeleteImage() error {
	imageID := i.GetImageID()
	if imageID == "" {
		return nil
	}

	if err := i.IBMPowerVSClient.DeleteImage(imageID); err != nil {
		record.Warnf(i.IBMPowerVSImage, "FailedDeleteImage", "Failed image deletion: %v", err)
		return fmt.Errorf("failed to delete PowerVS image %s: %w", imageID, err)
	}

	record.Eventf(i.IBMPowerVSImage, "SuccessfulDeleteImage", "Deleted Image %q", imageID)
	return nil
}

// DeleteImportJob will delete the image import job.
func (i *ImageScope) DeleteImportJob() error {
	jobID := i.GetJobID()
	if jobID == "" {
		return nil
	}

	if err := i.IBMPowerVSClient.DeleteJob(jobID); err != nil {
		record.Warnf(i.IBMPowerVSImage, "FailedDeleteImageImportJob", "Failed image import job deletion: %v", err)
		return fmt.Errorf("failed to delete COS image import job %s: %w", jobID, err)
	}

	record.Eventf(i.IBMPowerVSImage, "SuccessfulDeleteImageImportJob", "Deleted image import job %q", jobID)
	return nil
}

// SetReady will set the status as ready for the image.
func (i *ImageScope) SetReady() {
	i.IBMPowerVSImage.Status.Ready = true
}

// SetNotReady will set the status as not ready for the image.
func (i *ImageScope) SetNotReady() {
	i.IBMPowerVSImage.Status.Ready = false
}

// IsReady will return the status for the image.
func (i *ImageScope) IsReady() bool {
	return i.IBMPowerVSImage.Status.Ready
}

// SetImageID will set the id for the image.
func (i *ImageScope) SetImageID(id *string) {
	if id != nil {
		i.IBMPowerVSImage.Status.ImageID = *id
	}
}

// GetImageID will get the id for the image.
func (i *ImageScope) GetImageID() string {
	return i.IBMPowerVSImage.Status.ImageID
}

// SetImageState will set the state for the image.
func (i *ImageScope) SetImageState(status string) {
	i.IBMPowerVSImage.Status.ImageState = infrav1.PowerVSImageState(status)
}

// GetImageState will get the state for the image.
func (i *ImageScope) GetImageState() infrav1.PowerVSImageState {
	return i.IBMPowerVSImage.Status.ImageState
}

// SetJobID will set the id for the import image job.
func (i *ImageScope) SetJobID(id string) {
	i.IBMPowerVSImage.Status.JobID = id
}

// GetJobID will get the id for the import image job.
func (i *ImageScope) GetJobID() string {
	return i.IBMPowerVSImage.Status.JobID
}

// ensureImageUnique checks whether an image with the given name already exists
// in the workspace. Returns the existing reference if found, nil otherwise.
func (i *ImageScope) ensureImageUnique(imageName string) (*models.ImageReference, error) {
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

// getImportJob returns the current COS image import job for the workspace.
func (i *ImageScope) getImportJob() (*models.Job, error) {
	return i.IBMPowerVSClient.GetCosImages(i.workspaceID)
}
