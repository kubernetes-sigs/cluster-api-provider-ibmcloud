/*
Copyright 2019 The Kubernetes authors.

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

package machine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/softlayer/softlayer-go/datatypes"
	"github.com/softlayer/softlayer-go/sl"
	apicorev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	tokenapi "k8s.io/cluster-bootstrap/token/api"
	tokenutil "k8s.io/cluster-bootstrap/token/util"
	"k8s.io/klog"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clusterv1 "sigs.k8s.io/cluster-api/pkg/apis/cluster/v1alpha1"
	"sigs.k8s.io/cluster-api/pkg/util"

	ibmcloudv1 "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/apis/ibmcloud/v1alpha1"
	bootstrap "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/bootstrap"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/ibmcloud"
	ibmcloudclients "sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/ibmcloud/clients"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/cloud/ibmcloud/options"
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/record"
)

const (
	ProviderName = "ibmcloud"
	UserDataKey  = "userData"

	MachinePending  string = "Pending"
	MachineCreating string = "Creating"
	MachineRunning  string = "Running"
	MachineFailed   string = "Failed"
	MachineDeleting string = "Deleting"
)

// Actuator is responsible for performing machine reconciliation
type IBMCloudClient struct {
	params ibmcloud.ActuatorParams
	client client.Client
}

// NewActuator creates a new Actuator
func NewActuator(params ibmcloud.ActuatorParams) (*IBMCloudClient, error) {
	return &IBMCloudClient{
		params: params,
		client: params.Client,
	}, nil
}

// Create creates a machine and is invoked by the Machine Controller
func (ic *IBMCloudClient) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	klog.Infof("Creating machine %v for cluster %v.", machine.Name, cluster.Name)

	ic.updatePhase(ctx, machine, MachinePending)

	kubeClient := ic.params.KubeClient
	machineService, err := ibmcloudclients.NewInstanceServiceFromMachine(kubeClient, machine)
	if err != nil {
		return err
	}

	providerSpec, err := ibmcloudv1.MachineSpecFromProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	guest, err := ic.getGuest(machine)
	if err != nil {
		return err
	}
	if guest != nil {
		klog.Infof("Skipped creating a VM that already exists.\n")
		return nil
	}

	var userData []byte
	if providerSpec.UserDataSecret != nil {
		namespace := providerSpec.UserDataSecret.Namespace
		if namespace == "" {
			namespace = machine.Namespace
		}

		if providerSpec.UserDataSecret.Name == "" {
			return fmt.Errorf("UserDataSecret name must be provided")
		}

		userDataSecret, err := kubeClient.CoreV1().Secrets(namespace).Get(providerSpec.UserDataSecret.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}

		var ok bool
		userData, ok = userDataSecret.Data[UserDataKey]
		if !ok {
			return fmt.Errorf("Machine's userdata secret %v in namespace %v did not contain key %v", providerSpec.UserDataSecret.Name, namespace, UserDataKey)
		}
	}

	var userScriptRendered string
	if len(userData) > 0 {
		if util.IsControlPlaneMachine(machine) {
			userScriptRendered, err = masterStartupScript(cluster, machine, string(userData))
		} else {
			token, err := ic.createBootstrapToken()
			if err != nil {
				return fmt.Errorf("Failed to create token: %s", err)
			}
			userScriptRendered, err = nodeStartupScript(cluster, machine, token, string(userData))
		}
	}
	if err != nil {
		return err
	}

	ic.updatePhase(ctx, machine, MachineCreating)
	machineService.CreateGuest(cluster.Name, machine.Name, providerSpec, userScriptRendered)
	guest, err = ic.getGuest(machine)
	if err != nil {
		ic.updatePhase(ctx, machine, MachineFailed)
		return err
	}

	if guest == nil {
		ic.updatePhase(ctx, machine, MachineFailed)
		return fmt.Errorf("Guest %s does not exist after created in cluster %s", machine.Name, cluster.Name)
	}

	// FIXME: Temply set an empty machine status to pass delete machine check
	ext, err := ibmcloudv1.EncodeMachineStatus(&ibmcloudv1.IBMCloudMachineProviderStatus{})
	if err != nil {
		ic.updatePhase(ctx, machine, MachineFailed)
		return fmt.Errorf("Guest %s encode status failed in cluster %s", machine.Name, cluster.Name)
	}
	machine.Status.ProviderStatus = ext

	ic.updatePhase(ctx, machine, MachineRunning)
	record.Eventf(machine, "CreatedInstance", "Created new instance id: %d", *guest.Id)

	return ic.updateMachine(machine, strconv.Itoa(*guest.Id))
}

// Delete deletes a machine and is invoked by the Machine Controller
func (ic *IBMCloudClient) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	machineService, err := ibmcloudclients.NewInstanceServiceFromMachine(ic.params.KubeClient, machine)
	if err != nil {
		return err
	}

	guest, err := ic.getGuest(machine)
	if err != nil {
		return err
	}

	if guest == nil {
		klog.Infof("Skipped deleting %s that is already deleted.\n", machine.Name)
		return nil
	}

	ic.updatePhase(ctx, machine, MachineDeleting)

	err = machineService.DeleteGuest(*guest.Id)
	if err != nil {
		return fmt.Errorf("Guest delete failed %s", err)
	}

	record.Eventf(machine, "DeletedInstance", "Terminated instance %d", *guest.Id)

	return nil
}

func (ic *IBMCloudClient) validateGuest(guest *datatypes.Virtual_Guest) error {
	if guest.Domain == nil {
		return fmt.Errorf("Guest domain is empty")
	}

	if guest.OperatingSystemReferenceCode == nil {
		return fmt.Errorf("OS reference code is empty")
	}

	if guest.SupplementalCreateObjectOptions == nil || guest.SupplementalCreateObjectOptions.FlavorKeyName == nil {
		return fmt.Errorf("Guest flavor is empty")
	}

	if guest.HourlyBillingFlag == nil {
		return fmt.Errorf("Guest billing flag is empty")
	}

	if guest.Datacenter == nil || guest.Datacenter.Name == nil {
		return fmt.Errorf("Guest Data center is empty")
	}

	return nil
}

// Update updates a machine and is invoked by the Machine Controller
func (ic *IBMCloudClient) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	klog.Infof("Updating machine %v for cluster %v.", machine.Name, cluster.Name)

	// TODO(xunpan): node ref updating?

	guest, err := ic.getGuest(machine)
	if err != nil {
		return err
	}

	if guest == nil {
		return ic.Create(ctx, cluster, machine)
	}

	// FIXME: Temply set an empty machine status to pass delete machine check
	// This need to be here because the master node doesn't set Status in Create function
	ext, err := ibmcloudv1.EncodeMachineStatus(&ibmcloudv1.IBMCloudMachineProviderStatus{})
	if err != nil {
		ic.updatePhase(ctx, machine, MachineFailed)
		return fmt.Errorf("Guest %s encode status failed in cluster %s", machine.Name, cluster.Name)
	}
	machine.Status.ProviderStatus = ext

	ic.updatePhase(ctx, machine, MachineRunning)

	err = ic.updateMachine(machine, strconv.Itoa(*guest.Id))
	if err != nil {
		return err
	}

	if util.IsControlPlaneMachine(machine) {
		klog.Info("TODO: Master inplace update not yet implemented")
		return nil
	}

	machineSpec, err := ibmcloudv1.MachineSpecFromProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return err
	}

	err = ic.validateGuest(guest)
	if err != nil {
		return err
	}

	if *guest.Domain == machineSpec.Domain &&
		*guest.SupplementalCreateObjectOptions.FlavorKeyName == machineSpec.Flavor &&
		(*guest.OperatingSystemReferenceCode == machineSpec.OSReferenceCode ||
			// TODO(xunpan): we do not know what is the latest, so latest is fine currently
			strings.HasSuffix(machineSpec.OSReferenceCode, "_LATEST")) &&
		*guest.HourlyBillingFlag == machineSpec.HourlyBillingFlag &&
		*guest.Datacenter.Name == machineSpec.Datacenter {
		// TODO(xunpan)
		// currently, resource attribute updating triggers recreating a VM instance
		// any use case to change:
		//   - ssh key name
		//   - ssh key name
		//   - host name
		return nil
	}

	klog.Infof("Guest: Domain: %v, Flavor: %v, OSReferenceCode: %v, HourlyBillingFlag: %v, Datacenter: %v",
		machineSpec.Domain, *guest.SupplementalCreateObjectOptions.FlavorKeyName, *guest.OperatingSystemReferenceCode,
		machineSpec.HourlyBillingFlag, *guest.Datacenter.Name)
	klog.Infof("Machine: Domain: %v, Flavor: %v, OSReferenceCode: %v, HourlyBillingFlag: %v, Datacenter: %v",
		machineSpec.Domain, machineSpec.Flavor, machineSpec.OSReferenceCode,
		machineSpec.HourlyBillingFlag, machineSpec.Datacenter)

	klog.Infof("Recreating VM instances for machine %v of cluster %v.", machine.Name, cluster.Name)
	err = ic.Delete(ctx, cluster, machine)
	if err != nil {
		return err
	}

	return ic.Create(ctx, cluster, machine)
}

// Exists test for the existance of a machine and is invoked by the Machine Controller
func (ic *IBMCloudClient) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
	guest, err := ic.getGuest(machine)
	if err != nil {
		return false, err
	}

	if (guest != nil) && (machine.Spec.ProviderID == nil || *machine.Spec.ProviderID == "") {
		// TODO(xunpan): this does not work in ibm cloud if there is no `updatePhase`
		// check why related providers only set it in Exists but not update resource works
		providerID := fmt.Sprintf("ibmcloud:////%d", *guest.Id)
		machine.Spec.ProviderID = &providerID

		ic.updatePhase(ctx, machine, MachineRunning)
	}

	// Set the Machine NodeRef.
	if guest != nil && machine != nil && machine.Status.NodeRef == nil {
		nodeRef, err := ic.getNodeReference(machine.Name)
		if err == nil {
			machine.Status.NodeRef = nodeRef
			klog.Infof("Setting machne %v/%v noderef to %v", machine.Namespace, machine.Name, nodeRef.Name)
		} else {
			klog.Info("Cannot set nodeRef: ", err)
		}
	}

	return guest != nil, nil
}

func (ic *IBMCloudClient) getNodeReference(name string) (*apicorev1.ObjectReference, error) {
	getOpt := metav1.GetOptions{}
	node, err := ic.params.KubeClient.CoreV1().Nodes().Get(name, getOpt)
	if err != nil {
		return nil, errors.Errorf("%v set failed: %v", name, err)
	}

	return &apicorev1.ObjectReference{
		Kind:       "Node",
		APIVersion: apicorev1.SchemeGroupVersion.String(),
		Name:       node.Name,
		UID:        node.UID,
	}, nil
}

func (ic *IBMCloudClient) getGuest(machine *clusterv1.Machine) (*datatypes.Virtual_Guest, error) {
	machineService, err := ibmcloudclients.NewInstanceServiceFromMachine(ic.params.KubeClient, machine)
	if err != nil {
		return nil, err
	}

	providerSpec, err := ibmcloudv1.MachineSpecFromProviderSpec(machine.Spec.ProviderSpec)
	if err != nil {
		return nil, err
	}

	guest, err := machineService.GetGuest(machine.Name, providerSpec.Domain)
	if err != nil {
		return nil, err
	}
	return guest, nil
}

func (ic *IBMCloudClient) getIP(machine *clusterv1.Machine) (string, error) {
	guest, err := ic.getGuest(machine)
	if err != nil {
		return "", err
	}
	if guest == nil {
		return "", fmt.Errorf("Guest does not exist")
	}
	if guest.PrimaryIpAddress == nil {
		return "", fmt.Errorf("Guest IP does not exist")
	}

	return *guest.PrimaryIpAddress, nil
}

func (ic *IBMCloudClient) updateMachine(machine *clusterv1.Machine, id string) error {
	if machine.ObjectMeta.Annotations == nil {
		machine.ObjectMeta.Annotations = make(map[string]string)
	}

	ip, err := ic.getIP(machine)
	if err != nil {
		return err
	}
	machine.ObjectMeta.Annotations[ibmcloud.IBMCloudIPAnnotationKey] = ip

	if machine.Spec.ProviderID == nil || *machine.Spec.ProviderID == "" {
		providerID := fmt.Sprintf("ibmcloud:////%s", id)
		machine.Spec.ProviderID = &providerID
	}

	if err := ic.params.Client.Update(nil, machine); err != nil {
		return err
	}

	return nil
}

func (ic *IBMCloudClient) createBootstrapToken() (string, error) {
	token, err := tokenutil.GenerateBootstrapToken()
	if err != nil {
		return "", err
	}

	expiration := time.Now().UTC().Add(options.TokenTTL)
	tokenSecret, err := bootstrap.GenerateTokenSecret(token, expiration)
	if err != nil {
		klog.Fatalf("Unable to create token: %v", err)
	}

	err = ic.client.Create(context.TODO(), tokenSecret)
	if err != nil {
		return "", err
	}

	return tokenutil.TokenFromIDAndSecret(
		string(tokenSecret.Data[tokenapi.BootstrapTokenIDKey]),
		string(tokenSecret.Data[tokenapi.BootstrapTokenSecretKey]),
	), nil
}

func (ic *IBMCloudClient) updatePhase(ctx context.Context, machine *clusterv1.Machine, status string) {
	machine.Status.Phase = sl.String(status)
	err := ic.params.Client.Status().Update(ctx, machine)
	if err != nil {
		klog.Infof("Failed updating phase: %v", err)
	}
}
