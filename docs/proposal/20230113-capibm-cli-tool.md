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

### 1. capibmadm powervs images

* **Environment Variables:**

  * IBMCLOUD_API_KEY: IBM Cloud api key of user.
  * SERVICE_INSTANCE_ID: PowerVS service instance id.


* **Arguments:**
  * --service-instance-id: Power VS service instance id to query to get images.


### 2. capibmadm powervs image-import

* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.
  * SERVICE_INSTANCE_ID: PowerVS service instance id.


* **Arguments:**
  * _Similar to [pvsadm image import](https://github.com/ppc64le-cloud/pvsadm/blob/824f87baebd430b26ed8d3ec517077a9d5b5824b/cmd/image/import/import.go#L63) command_


### 3. capibmadm powervs network-create <network_name>

* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.
  * SERVICE_INSTANCE_ID: PowerVS service instance id.


* **Arguments:**
  * --dns-servers: Comma separated list of DNS Servers to use for this network


### 4. capibmadm powervs network-delete <network_name>

* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.
  * SERVICE_INSTANCE_ID: PowerVS service instance id.


* **Arguments:**

  * force: Boolean flag to force delete network by deleting all the open ports attached to network.

### 5. capibmadm powervs port-create

* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.
  * SERVICE_INSTANCE_ID: PowerVS service instance id.


* **Arguments:**
  * _Similar to [pvsadm create port](https://github.com/ppc64le-cloud/pvsadm/blob/824f87baebd430b26ed8d3ec517077a9d5b5824b/cmd/create/port/port.go#L34) command_


### 6. capibmadm powervs port-delete

* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.
  * SERVICE_INSTANCE_ID: PowerVS service instance id.


* **Arguments:**
  * _Similar to [pvsadm delete port](https://github.com/ppc64le-cloud/pvsadm/blob/824f87baebd430b26ed8d3ec517077a9d5b5824b/cmd/delete/port/port.go#L33) command_


### 7. capibmadm powervs ports

* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.
  * SERVICE_INSTANCE_ID: PowerVS service instance id.


* **Arguments:**
  * _Similar to [pvsadm get port](https://github.com/ppc64le-cloud/pvsadm/blob/824f87baebd430b26ed8d3ec517077a9d5b5824b/cmd/get/ports/ports.go#L33) command_

### 7. capibmadm powervs key-create
* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.
  * SERVICE_INSTANCE_ID: PowerVS service instance id.


* **Arguments:**
    * Similar to ibmcloud pi key-create

### 8. capibmadm powervs keys
* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.
  * SERVICE_INSTANCE_ID: PowerVS service instance id.


* **Arguments:**
  * _Similar to ibmcloud pi keys_

### 9. capibmadm vpc key-create
* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.


* **Arguments:**
  * Similar to ibmcloud is key-create

### 10. capibmadm vpc keys
* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.


* **Arguments:**
  * Similar to ibmcloud is keys


### 11. capibmadm vpc images
* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.


* **Arguments:**
  * resource-group-name: Resource group name to list images
  * region: Region name to list images

### 12. capibmadm vpc image-import
* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.

* **Arguments:**
  * _Similar to ibmcloud is image-create_


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
              |   |   |-- ssh
              |   |-- vpc
              |   |   |-- image
              |   |   |-- ssh
              |   |-- root.go
              |-- options
              |-- utility
              |-- main.go
```
