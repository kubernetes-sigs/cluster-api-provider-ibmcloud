## VPC key Commands

 ### 1. capibmadm vpc key create

 #### Usage: 
 Create a key in the VPC environment.

 #### Environmental Variable:
 IBMCLOUD_API_KEY: IBM Cloud api key.

 #### Arguments:

  --name: The name of the key. 

  --resource-group-name: Optional VPC resource group name.

  --region: VPC region.

 Either of the arguments need to be provided:

  --public-key: Public key string within a double quotation marks. For example, "ssh-rsa AAA... ".

  --key-path: The absolute path to the VPC key file.


 #### Example:
 ```shell
 export IBMCLOUD_API_KEY=<api-key>

 capibmadm vpc key create --name <key-name> --region <region> --resource-group-name <resource-group-name> --public-key "<public-key-string>"

 capibmadm vpc key create --name <key-name> --region <region> --resource-group-name <resource-group-name> --key-path <path/to/vpc/key>
 ```

 ### 2. capibmadm vpc key delete

 #### Usage:
 Delete a key in the VPC environment.

 #### Environmental Variable:
 IBMCLOUD_API_KEY: IBM Cloud api key.

 #### Arguments:

  --name: The name of the key.

  --region: VPC region.

 #### Example:
 ```shell
 export IBMCLOUD_API_KEY=<api-key>
 capibmadm vpc key delete --name <key-name> --region <region>
 ```