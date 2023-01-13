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
  
### 2. capibmadm powervs image import

* **Arguments:**
  * _Similar to [pvsadm image import](https://github.com/ppc64le-cloud/pvsadm/blob/824f87baebd430b26ed8d3ec517077a9d5b5824b/cmd/image/import/import.go#L63) command_


### 3. capibmadm powervs network create <network_name>

* **Arguments:**
  * --dns-servers: Comma separated list of DNS Servers to use for this network.


### 4. capibmadm powervs network delete <network_name>

* **Arguments:**
  * force: Boolean flag to force delete network by deleting all the open ports attached to network.

### 5. capibmadm powervs network list


### 6. capibmadm powervs port create

* **Arguments:**
  * _Similar to [pvsadm create port](https://github.com/ppc64le-cloud/pvsadm/blob/824f87baebd430b26ed8d3ec517077a9d5b5824b/cmd/create/port/port.go#L34) command_


### 7. capibmadm powervs port delete

* **Arguments:**
  * _Similar to [pvsadm delete port](https://github.com/ppc64le-cloud/pvsadm/blob/824f87baebd430b26ed8d3ec517077a9d5b5824b/cmd/delete/port/port.go#L33) command_


### 8. capibmadm powervs port list

* **Arguments:**
  * _Similar to [pvsadm get port](https://github.com/ppc64le-cloud/pvsadm/blob/824f87baebd430b26ed8d3ec517077a9d5b5824b/cmd/get/ports/ports.go#L33) command_

### 9. capibmadm powervs key create

* **Arguments:**
    * _Similar to ibmcloud pi key-create_

### 10. capibmadm powervs key list

* **Arguments:**
  * _Similar to ibmcloud pi keys_

## VPC Commands
* **Environment Variables:**
  * IBMCLOUD_API_KEY: IBM Cloud api key of user.

### 1. capibmadm vpc key create

* **Arguments:**
  * _Similar to ibmcloud is key-create_

### 2. capibmadm vpc key list

* **Arguments:**
  * _Similar to ibmcloud is keys_


### 3. capibmadm vpc image list

* **Arguments:**
  * resource-group-name: Resource group name to list images
  * region: Region name to list images

### 4. capibmadm vpc image import

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
