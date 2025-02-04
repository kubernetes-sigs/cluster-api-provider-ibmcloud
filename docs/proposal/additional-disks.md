# Adding additional volumes to Nodes

[Github Issue](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/issues/1920)

## Summary
We want the ability to add additional Volumes to our Machine nodes.

## Proposal
We will need to make changes to both the API and the controller logic.
With regards to the API, we will need to add a new `AdditionalVolumes` field to the Machine Spec to contain all the different volumes that are added to the node, the field will be a slice of type `VPCVolume`. As an initial design we can keep the field [append-only](https://kubernetes.io/blog/2022/09/29/enforce-immutability-using-cel/#append-only-list-of-containers).

### API Changes
The Additional Volumes field will be a part of the `IBMVPCMachineSpec` struct and will be defined as follows:
```go
    // AdditionalVolumes is the list of additional volumes attached to the disk
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:MaxItems=10
	// +kubebuilder:validation:XValidation:rule="oldSelf.all(x, x in self)",message="Values may only be added"
	AdditionalVolumes []*VPCVolume `json:"additionalVolumes,omitempty"`
```

The `VPCVolume` struct is [already present](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/blob/3bf33db00c0a46dfc2b78861e8519072fa51b441/api/v1beta2/ibmvpcmachine_types.go#L99) in the API spec and is currently being used to describe the boot disks.

A sample IBMVPCMachineSpec instance with AdditionalDisks would look like the following:
```yaml
# apiVersion, kind, metadata...
spec:
  additionalVolumes:
  # Additional Volume 1
  - name: GeneralVolume
    profile: "general-purpose" # Using general-purpose as the volume profile here allows us to use the defaults associated with this template
  # Additional Volume 2
  - iops: 10000
    name: CustomVolume
    profile: "custom" # Using a custom profile allows us more freedom when choosing the volume spec
    sizeGiB: 120
  # Standard Boot Volume, this is already a part of the spec
  bootVolume:
    name: BootVolume
    profile: "general-purpose"
    sizeGiB: 100
  # Other IBMVPCMachine.Spec fields
```

### Controller flow
The flow for creating Additional Volumes will be similar to creating the Boot Volume.
We will need to add checks in the admission and mutation webhooks to validate any changes to the field like we do for `VPCVolume`.
We can allow the field be unset initially so if a user wants to add Volumes after provisioning a machine, they will be able to.

The Machine create flow will be as follows:
If a user does not add Additional Volumes while creating the Machine, the standard create flow will be followed.
If they add Additional Volumes when creating the Machine, the Machine reconciler will also create the Additional Volumes and attach them to the machine.

The Machine update flow will be as follows:
The `AdditionalVolumes` field will be append-only so a user will not be able to remove already created volumes. If a user adds more volumes to the slice, those Volumes will be provisioned and then attached to the machine.

The Delete flow will not change much, the Additional Volumes' deletion flow will be similar to the Boot Volume's. 

### Limitation
The biggest limitation of the current approach will be that the `AdditionalVolumes` field will be append-only and therefore there won't be a way to delete any Additional Volumes that are provisioned without deleting the entire machine.
This limitation can be addressed later by enhancing the controller flow to manage update scenarios where Volumes are removed from the `AdditionalVolumes` slice. This would also require the append-only nature of the `AdditionalVolumes` field to change.
