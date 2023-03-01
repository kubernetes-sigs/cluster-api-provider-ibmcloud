## VPC SSH key Commands

### 1. capibmadm vpc key list

#### Usage:
List SSH keys in given VPC region.

#### Environmental Variable:
IBMCLOUD_API_KEY: IBM Cloud API key.

#### Arguments:
--region: VPC region.

--resource-group-name: IBM Cloud resource group name.

#### Example:
```shell
export IBMCLOUD_API_KEY=<api-key>
capibmadm vpc key list --region <region> --resource-group-name <resource-group>
```