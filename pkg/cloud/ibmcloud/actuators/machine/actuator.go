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

	"github.com/softlayer/softlayer-go/datatypes"
	"github.com/softlayer/softlayer-go/sl"
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
	"sigs.k8s.io/cluster-api-provider-ibmcloud/pkg/record"
)

const (
	ProviderName = "ibmcloud"
	UserDataKey  = "userData"

	TokenTTL = 60 * time.Minute

	MachinePending  string = "Pending"
	MachineCreating string = "Creating"
	MachineRunning  string = "Running"
	MachineFailed   string = "Failed"
	MachineDeleting string = "Deleting"
)

// Actuator is responsible for performing machine reconciliation
type IbmCloudClient struct {
	params ibmcloud.ActuatorParams
	client client.Client
}

// NewActuator creates a new Actuator
func NewActuator(params ibmcloud.ActuatorParams) (*IbmCloudClient, error) {
	return &IbmCloudClient{
		params: params,
		client: params.Client,
	}, nil
}

// Create creates a machine and is invoked by the Machine Controller
func (ic *IbmCloudClient) Create(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
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
				return fmt.Errorf("Failed to create toke: %s", err)
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

	ic.updatePhase(ctx, machine, MachineRunning)
	record.Eventf(machine, "CreatedInstance", "Created new instance id: %d", *guest.Id)

	return ic.updateMachine(machine, strconv.Itoa(*guest.Id))
}

// Delete deletes a machine and is invoked by the Machine Controller
func (ic *IbmCloudClient) Delete(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
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

// Update updates a machine and is invoked by the Machine Controller
func (ic *IbmCloudClient) Update(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) error {
	klog.Infof("Updating machine %v for cluster %v.", machine.Name, cluster.Name)

	// TODO(xunpan): node ref updating?

	guest, err := ic.getGuest(machine)
	if err != nil {
		return err
	}

	if guest == nil {
		return ic.Create(ctx, cluster, machine)
	}

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

	if guest.Domain == nil ||
		guest.MaxMemory == nil ||
		guest.OperatingSystemReferenceCode == nil ||
		guest.LocalDiskFlag == nil ||
		guest.HourlyBillingFlag == nil ||
		guest.Datacenter.Name == nil ||
		guest.StartCpus == nil {
		klog.Info("Guest does not contain full information.")
		return nil
	}

	if *guest.Domain == machineSpec.Domain &&
		*guest.MaxMemory == machineSpec.MaxMemory &&
		(*guest.OperatingSystemReferenceCode == machineSpec.OSReferenceCode ||
			// TODO(xunpan): we do not know what is the latest, so latest is fine currently
			strings.HasSuffix(machineSpec.OSReferenceCode, "_LATEST")) &&
		*guest.LocalDiskFlag == machineSpec.LocalDiskFlag &&
		*guest.HourlyBillingFlag == machineSpec.HourlyBillingFlag &&
		*guest.Datacenter.Name == machineSpec.Datacenter &&
		*guest.StartCpus == machineSpec.StartCpus {
		// TODO(xunpan)
		// currently, resource attribute updating triggers recreating a VM instance
		// any use case to change:
		//   - ssh key name
		//   - ssh key name
		//   - host name
		return nil
	}

	klog.Infof("Guest: Domain: %v, MaxMemory: %v, OSReferenceCode: %v, LocalDiskFlag: %v, HourlyBillingFlag: %v, Datacenter: %v, StartCpus: %v",
		machineSpec.Domain, *guest.MaxMemory, *guest.OperatingSystemReferenceCode, *guest.LocalDiskFlag,
		machineSpec.HourlyBillingFlag, *guest.Datacenter.Name, *guest.StartCpus)
	klog.Infof("Machine: Domain: %v, MaxMemory: %v, OSReferenceCode: %v, LocalDiskFlag: %v, HourlyBillingFlag: %v, Datacenter: %v, StartCpus: %v",
		machineSpec.Domain, machineSpec.MaxMemory, machineSpec.OSReferenceCode, machineSpec.LocalDiskFlag,
		machineSpec.HourlyBillingFlag, machineSpec.Datacenter, machineSpec.StartCpus)

	klog.Infof("Recreating VM instances for machine %v of cluster %v.", machine.Name, cluster.Name)
	err = ic.Delete(ctx, cluster, machine)
	if err != nil {
		return err
	}

	return ic.Create(ctx, cluster, machine)
}

// Exists test for the existance of a machine and is invoked by the Machine Controller
func (ic *IbmCloudClient) Exists(ctx context.Context, cluster *clusterv1.Cluster, machine *clusterv1.Machine) (bool, error) {
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

	return guest != nil, nil
}

func (ic *IbmCloudClient) getGuest(machine *clusterv1.Machine) (*datatypes.Virtual_Guest, error) {
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

func (ic *IbmCloudClient) getIP(machine *clusterv1.Machine) (string, error) {
	guest, err := ic.getGuest(machine)
	if err != nil {
		return "", err
	}
	if guest == nil {
		return "", fmt.Errorf("Guest does not exist")
	}
	if guest.PrimaryIpAddress == nil {
		klog.Infof("Guest IP does not exist")
		return "", nil
	}

	return *guest.PrimaryIpAddress, nil
}

func (ic *IbmCloudClient) updateMachine(machine *clusterv1.Machine, id string) error {
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

func (ic *IbmCloudClient) createBootstrapToken() (string, error) {
	token, err := tokenutil.GenerateBootstrapToken()
	if err != nil {
		return "", err
	}

	expiration := time.Now().UTC().Add(TokenTTL)
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

func (ic *IbmCloudClient) updatePhase(ctx context.Context, machine *clusterv1.Machine, status string) {
	machine.Status.Phase = sl.String(status)
	err := ic.params.Client.Status().Update(ctx, machine)
	if err != nil {
		klog.Infof("Failed updating phase: %v", err)
	}
}
