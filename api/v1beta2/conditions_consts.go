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

package v1beta2

import (
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
)

// IBMPowerVSMachine's InstanceReady condition and corresponding reasons that will be used in v1Beta2 API version.
const (
	// IBMPowerVSMachineReadyV1Beta2Condition is true if the IBMPowerVSMachine's deletionTimestamp is not set, IBMPowerVSMachine's
	// IBMPowerVSMachineInstanceReadyV1Beta2Condition is true.
	IBMPowerVSMachineReadyV1Beta2Condition = capiv1beta1.ReadyV1Beta2Condition

	// IBMPowerVSMachineReadyV1Beta2Reason surfaces when the IBMPowerVSMachine readiness criteria is met.
	IBMPowerVSMachineReadyV1Beta2Reason = capiv1beta1.ReadyV1Beta2Reason

	// IBMPowerVSMachineNotReadyV1Beta2Reason surfaces when the IBMPowerVSMachine readiness criteria is not met.
	IBMPowerVSMachineNotReadyV1Beta2Reason = capiv1beta1.NotReadyV1Beta2Reason

	// IBMPowerVSMachineReadyUnknownV1Beta2Reason surfaces when at least one IBMPowerVSMachine readiness criteria is unknown
	// and no IBMPowerVSMachine readiness criteria is not met.
	IBMPowerVSMachineReadyUnknownV1Beta2Reason = capiv1beta1.ReadyUnknownV1Beta2Reason
)

const (
	// IBMPowerVSMachineInstanceReadyV1Beta2Condition documents the status of the instance that is controlled
	// by the IBMPowerVSMachine.
	IBMPowerVSMachineInstanceReadyV1Beta2Condition = "InstanceReady"

	// IBMPowerVSMachineInstanceReadyV1Beta2Reason surfaces when the instance that is controlled
	// by the IBMPowerVSMachine is ready.
	IBMPowerVSMachineInstanceReadyV1Beta2Reason = "InstanceReady"

	// IBMPowerVSMachineInstanceNotReadyV1Beta2Reason surfaces when the instance that is controlled
	// by the IBMPowerVSMachine is not ready.
	IBMPowerVSMachineInstanceNotReadyV1Beta2Reason = "InstanceNotReady"

	// IBMPowerVSMachineInstanceWaitingForClusterInfrastructureReadyV1Beta2Reason documents the virtual machine that is controller by
	// IBMPowerVSMachine waiting for the cluster infrastructure to be ready.
	// TODO: Use when CAPI version is updated: IBMPowerVSMachineInstanceWaitingForClusterInfrastructureReadyV1Beta2Reason = capiv1beta1.WaitingForClusterInfrastructureReadyV1Beta2Reason.
	IBMPowerVSMachineInstanceWaitingForClusterInfrastructureReadyV1Beta2Reason = "WaitingForClusterInfrastructureReady"

	// IBMPowerVSMachineInstanceWaitingForControlPlaneInitializedV1Beta2Reason documents the virtual machine that is controller by IBMPowerVSMachine waiting
	// for the control plane to be initialized.
	// TODO: Use when CAPI version is updated: IBMPowerVSMachineInstanceWaitingForControlPlaneInitializedV1Beta2Reason = capiv1beta1.WaitingForControlPlaneInitializedV1Beta2Reason.
	IBMPowerVSMachineInstanceWaitingForControlPlaneInitializedV1Beta2Reason = "WaitingForControlPlaneInitialized"

	// IBMPowerVSMachineInstanceWaitingForBootstrapDataV1Beta2Reason documents the virtual machine that is controller by IBMPowerVSMachine waiting for the bootstrap
	// data to be ready.
	// TODO: Use when CAPI version is updated: IBMPowerVSMachineInstanceWaitingForBootstrapDataV1Beta2Reason = capiv1beta1.WaitingForBootstrapDataV1Beta2Reason.
	IBMPowerVSMachineInstanceWaitingForBootstrapDataV1Beta2Reason = "WaitingForBootstrapData"

	// IBMPowerVSMachineInstanceDeletingV1Beta2Reason surfaces when the virtual machine controller by IBMPowerVSMachine is deleting.
	IBMPowerVSMachineInstanceDeletingV1Beta2Reason = capiv1beta1.DeletingV1Beta2Reason

	// IBMPowerVSMachineInstanceLoadBalancerConfigurationFailedV1Beta2Reason surfaces when configuring the virtual machine IP to load balancer fails.
	IBMPowerVSMachineInstanceLoadBalancerConfigurationFailedV1Beta2Reason = "LoadBalancerConfigurationFailed"

	// IBMPowerVSMachineInstanceWaitingForNetworkAddressV1Beta2Reason surfaces when the PowerVS instance that is controlled
	// by the IBMPowerVSMachine waiting for the machine network settings to be reported after machine being powered on.
	IBMPowerVSMachineInstanceWaitingForNetworkAddressV1Beta2Reason = "WaitingForNetworkAddress"
)

const (
	// InstanceProvisionFailedReason used for failures during instance provisioning.
	InstanceProvisionFailedReason = "InstanceProvisionFailed"
	// WaitingForClusterInfrastructureReason used when machine is waiting for cluster infrastructure to be ready before proceeding.
	WaitingForClusterInfrastructureReason = "WaitingForClusterInfrastructure"
	// WaitingForBootstrapDataReason used when machine is waiting for bootstrap data to be ready before proceeding.
	WaitingForBootstrapDataReason = "WaitingForBootstrapData"
)

const (
	// InstanceStoppedReason instance is in a stopped state.
	InstanceStoppedReason = "InstanceStopped"

	// InstanceErroredReason instance is in a errored state.
	InstanceErroredReason = "InstanceErrored"

	// InstanceNotReadyReason used when the instance is in a not ready state.
	InstanceNotReadyReason = "InstanceNotReady"

	// InstanceStateUnknownReason used when the instance is in a unknown state.
	InstanceStateUnknownReason = "InstanceStateUnknown"
)

const (
	// InstanceReadyCondition reports on current status of the instance. Ready indicates the instance is in a Running state.
	InstanceReadyCondition capiv1beta1.ConditionType = "InstanceReady"
)

const (
	// WaitingForIBMPowerVSImageReason used when machine is waiting for powervs image to be ready before proceeding.
	WaitingForIBMPowerVSImageReason = "WaitingForIBMPowerVSImage"
)

const (
	// ImageNotReadyReason used when the image is in a queued state.
	ImageNotReadyReason = "ImageNotReady"

	// ImageImportFailedReason used when the image import is failed.
	ImageImportFailedReason = "ImageImportFailed"

	// ImageReconciliationFailedReason used when an error occurs during VPC Custom Image reconciliation.
	ImageReconciliationFailedReason = "ImageReconciliationFailed"
)

const (
	// ImageReadyCondition reports on current status of the image. Ready indicates the image is in a active state.
	ImageReadyCondition capiv1beta1.ConditionType = "ImageReady"

	// ImageImportedCondition reports on current status of the image import job. Ready indicates the import job is finished.
	ImageImportedCondition capiv1beta1.ConditionType = "ImageImported"
)

const (
	// LoadBalancerNotReadyReason used when cluster is waiting for load balancer to be ready before proceeding.
	LoadBalancerNotReadyReason = "LoadBalancerNotReady"
)

const (
	// ServiceInstanceReadyCondition reports on the successful reconciliation of a Power VS workspace.
	ServiceInstanceReadyCondition capiv1beta1.ConditionType = "ServiceInstanceReady"
	// ServiceInstanceReconciliationFailedReason used when an error occurs during workspace reconciliation.
	ServiceInstanceReconciliationFailedReason = "ServiceInstanceReconciliationFailed"

	// NetworkReadyCondition reports on the successful reconciliation of a Power VS network.
	NetworkReadyCondition capiv1beta1.ConditionType = "NetworkReady"
	// NetworkReconciliationFailedReason used when an error occurs during network reconciliation.
	NetworkReconciliationFailedReason = "NetworkReconciliationFailed"

	// VPCSecurityGroupReadyCondition reports on the successful reconciliation of a VPC.
	VPCSecurityGroupReadyCondition capiv1beta1.ConditionType = "VPCSecurityGroupReady"
	// VPCSecurityGroupReconciliationFailedReason used when an error occurs during VPC reconciliation.
	VPCSecurityGroupReconciliationFailedReason = "VPCSecurityGroupReconciliationFailed"

	// VPCReadyCondition reports on the successful reconciliation of a VPC.
	VPCReadyCondition capiv1beta1.ConditionType = "VPCReady"
	// VPCReconciliationFailedReason used when an error occurs during VPC reconciliation.
	VPCReconciliationFailedReason = "VPCReconciliationFailed"

	// VPCSubnetReadyCondition reports on the successful reconciliation of a VPC subnet.
	VPCSubnetReadyCondition capiv1beta1.ConditionType = "VPCSubnetReady"
	// VPCSubnetReconciliationFailedReason used when an error occurs during VPC subnet reconciliation.
	VPCSubnetReconciliationFailedReason = "VPCSubnetReconciliationFailed"

	// TransitGatewayReadyCondition reports on the successful reconciliation of a Power VS transit gateway.
	TransitGatewayReadyCondition capiv1beta1.ConditionType = "TransitGatewayReady"
	// TransitGatewayReconciliationFailedReason used when an error occurs during transit gateway reconciliation.
	TransitGatewayReconciliationFailedReason = "TransitGatewayReconciliationFailed"

	// LoadBalancerReadyCondition reports on the successful reconciliation of a Power VS network.
	LoadBalancerReadyCondition capiv1beta1.ConditionType = "LoadBalancerReady"
	// LoadBalancerReconciliationFailedReason used when an error occurs during loadbalancer reconciliation.
	LoadBalancerReconciliationFailedReason = "LoadBalancerReconciliationFailed"

	// COSInstanceReadyCondition reports on the successful reconciliation of a COS instance.
	COSInstanceReadyCondition capiv1beta1.ConditionType = "COSInstanceCreated"
	// COSInstanceReconciliationFailedReason used when an error occurs during COS instance reconciliation.
	COSInstanceReconciliationFailedReason = "COSInstanceCreationFailed"
)

const (
	// CreateInfrastructureAnnotation is the name of an annotation that indicates if
	// Power VS infrastructure should be created as a part of cluster creation.
	CreateInfrastructureAnnotation = "powervs.cluster.x-k8s.io/create-infra"
)
