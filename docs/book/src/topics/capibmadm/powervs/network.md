## Power VS Network Commands

### 1. capibmadm powervs network create

#### Usage:
Create Power VS network.

#### Environmental Variable:
IBMCLOUD_API_KEY: IBM Cloud api key.

#### Arguments:
--service-instance-id: Power VS service instance id.

--dns-servers: Comma separated list of DNS Servers to use for this network, Defaults to 8.8.8.8, 9.9.9.9.

#### Example:
```shell
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs network create <network-name> --service-instance-id <service-instance-id>
```



### 2. capibmadm powervs network list

#### Usage:
List PowerVS networks.

#### Environmental Variable:
IBMCLOUD_API_KEY: IBM Cloud api key.

#### Arguments:
--service-instance-id: PowerVS service instance id.

--zone: PowerVS service instance zone.

#### Example:
```shell
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs network list --service-instance-id <service-instance-id> --zone <zone>
```
