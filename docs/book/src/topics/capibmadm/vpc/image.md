## PowerVS VPC Commands

### 1. capibmadm vpc image list

#### Usage:
List images in given VPC region.

#### Environmental Variable:
IBMCLOUD_API_KEY: IBM Cloud API key.

#### Arguments:
--region: VPC region.

--resource-group-name: IBM Cloud resource group name.

#### Example:
```shell
export IBMCLOUD_API_KEY=<api-key>
capibmadm vpc image list --region <region> --resource-group-name <resource-group>
```