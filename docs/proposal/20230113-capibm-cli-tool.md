# CLI tool for Cluster API Provider IBM Cloud

## Summary

This proposal aims to provide a cli tool to help users to perform various IBM Cloud operations which helps in creating and
managing Power VS or VPC workload clusters.

## Motivation

In order to create IBM Cloud Power VS cluster using cluster api as a prerequisite we need to create Power VS network, port and need import the appropriate image to Power VS service instance, currently we need to make use of different tools(ibmcloud, pvsadm cli) to achieve this.

The goal is to create a single unified cli tool to handle all the operations required prior creating a kubernetes cluster using cluster-api-provider-ibmcloud.

## Proposed commands

Command will be of following format

```shell
capibmadm <provider> <command> <subcommand>
```
Here provider can be either vpc or powervs

## Power VS Commands

**Environment Variables:**

  * IBMCLOUD_API_KEY: IBM Cloud api key of user.

**Arguments:**
  * --service-instance-id: Power VS service instance id.

### 1. capibmadm powervs image list

**Arguments:**

* --zone: PowerVS service instance zone.
  
### 2. capibmadm powervs image import

**Arguments:**

* --bucket: Cloud Object Storage bucket name.

* --bucket-region: Cloud Object Storage bucket location.

* --object: Cloud Object Storage object name.

* --accesskey: Cloud Object Storage HMAC access key.

* --secretkey: Cloud Object Storage HMAC secret key.

* --name: Name to PowerVS imported image.

* --public-bucket: Cloud Object Storage public bucket.

* --watch-timeout: watch timeout.

* --pvs-storagetype: PowerVS Storage type, accepted values are [tier1, tier3]..

### 3. capibmadm powervs network create <network_name>

**Arguments:**

* --cidr: The network CIDR. Required for private network type.

* --name: The name of the network.

* --public: Public (pub-vlan) network type (default true)

* --private: Private (vlan) network type (default false)

* --gateway: The gateway ip address.

* --dns-servers: Comma separated list of DNS Servers to use for this network, Defaults to 8.8.8.8, 9.9.9.9.

* --ip-ranges: Comma separated IP Address Ranges.

* --jumbo: Enable MTU Jumbo Network.

### 4. capibmadm powervs network delete <network_name>

**Arguments:**

* --zone: PowerVS service instance zone.

* --network: Network ID or Name.

### 5. capibmadm powervs network list

**Arguments:**

* --zone: PowerVS service instance zone.

### 6. capibmadm powervs port create

**Arguments:**

* --zone: PowerVS service instance zone.

* --network: Network ID/ Network Name.

* --description: Description of the port.

* --ip-address: The requested IP address of this port

### 7. capibmadm powervs port delete

**Arguments:**

* --zone: PowerVS zone.

* --port-id: ID of network port.

* --network: Network ID or Name.

### 8. capibmadm powervs port list

**Arguments:**

* --zone: PowerVS zone.

* --network: Network ID or Name.

### 9. capibmadm powervs key create

**Arguments:**

* --zone: PowerVS zone.

* --name: The name of the SSH key.

Either of the arguments need to be provided:

* --key: SSH RSA key string within a double quotation marks. For example, “ssh-rsa AAA... “.

* --key-path: The absolute path to the SSH key file.

### 10. capibmadm powervs key delete

**Arguments:**

* --zone: PowerVS zone.

* --name: The name of the SSH key.

### 11. capibmadm powervs key list

**Arguments:**

* --zone: PowerVS zone.

## VPC Commands

* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.

* **Arguments:**
  * --region: VPC region.

### 1. capibmadm vpc key create

**Arguments:**

* --name: The name of the key.

* --resource-group-name: VPC resource group name.

* --region: VPC region.

Either of the arguments need to be provided:

* --public-key: Public key string within a double quotation marks. For example, “ssh-rsa AAA... “.

* --key-path: The absolute path to the SSH key file.

### 2. capibmadm vpc key delete

**Arguments:**

* --name: The name of the key to delete.

### 3. capibmadm vpc key list

**Arguments:**

* --resource-group-name: IBM Cloud resource group name.


### 3. capibmadm vpc image list

**Arguments:**

* --name: The name of the key.

### 4. capibmadm vpc image import

**Arguments:**

* --resource-group-name: IBM Cloud resource group name.


##  Directory structure
```
  |-- cluster-api-provider-ibmcloud
      |-- cmd
          |-- capibmadm
              |-- cmd
              |   |-- powervs
              |   |   |-- image
              |   |   |-- network
              |   |   |-- port
              |   |   |-- key
              |   |-- vpc
              |   |   |-- image
              |   |   |-- key
              |   |-- root.go
              |-- options
              |-- utils
              |-- main.go
```
