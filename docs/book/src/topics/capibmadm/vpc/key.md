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

 ### 2. capibmadm vpc key create

 #### Usage: 
 Create a key in the VPC environment.

 #### Environmental Variable:
 IBMCLOUD_API_KEY: IBM Cloud API key.

 #### Arguments:

  --name: The name of the key. 

  --resource-group-name: VPC resource group name.

  --region: VPC region.

 Either of the arguments need to be provided:

  --public-key: Public key string within a double quotation marks. For example, "ssh-rsa AAA... ".

  --key-path: The absolute path to the SSH key file.


 #### Example:
 ```shell
 export IBMCLOUD_API_KEY=<api-key>

 capibmadm vpc key create --name <key-name> --region <region> --public-key "<public-key-string>"

 capibmadm vpc key create --name <key-name> --region <region> --key-path <path/to/ssh/key>
 ```

 ### 3. capibmadm vpc key delete

 #### Usage:
 Delete a key in the VPC environment.

 #### Environmental Variable:
 IBMCLOUD_API_KEY: IBM Cloud API key.

 #### Arguments:

  --name: The name of the key.

  --region: VPC region.

 #### Example:
 ```shell
 export IBMCLOUD_API_KEY=<api-key>
 capibmadm vpc key delete --name <key-name> --region <region>
 ```
