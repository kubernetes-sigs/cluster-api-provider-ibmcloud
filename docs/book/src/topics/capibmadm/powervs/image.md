## PowerVS Image Commands

### 1. capibmadm powervs image list

#### Usage:
List PowerVS images.

#### Environmental Variable:
IBMCLOUD_API_KEY: IBM Cloud API key.

#### Arguments:
--service-instance-id: PowerVS service instance id.

--zone: PowerVS service instance zone.


#### Example:
```shell
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs image list --service-instance-id <service-instance-id> --zone <zone>
```