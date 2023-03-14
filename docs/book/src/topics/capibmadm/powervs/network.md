## PowerVS Network Commands

### 1. capibmadm powervs network create

#### Usage:
Create PowerVS network.

#### Environmental Variable:
IBMCLOUD_API_KEY: IBM Cloud API key.

#### Arguments:
--service-instance-id: PowerVS service instance id.

--cidr: The network CIDR. Required for private network type.

--name: The name of the network.

--public: Public (pub-vlan) network type (default true)

--private: Private (vlan) network type (default false)

--gateway: The gateway ip address.

--dns-servers: Comma separated list of DNS Servers to use for this network, Defaults to 8.8.8.8, 9.9.9.9.

--ip-ranges: Comma separated IP Address Ranges.

--jumbo: Enable MTU Jumbo Network.


#### Example:
```shell
export IBMCLOUD_API_KEY=<api-key>
# Public network:
capibmadm powervs network create --public --service-instance-id <service-instance-id> --zone <zone>
# Private network:
capibmadm powervs network create --private --cidr <cidr> --service-instance-id <service-instance-id> --zone <zone>
# Private network with ip address ranges:
capibmadm powervs network create --private --cidr <cidr> --ip-ranges <start-ip>-<end-ip>,<start-ip>-<end-ip> --service-instance-id <service-instance-id> --zone <zone>
```


### 2. capibmadm powervs network delete

#### Usage:
Delete PowerVS network.

#### Environmental Variable:
IBMCLOUD_API_KEY: IBM Cloud API key.

#### Arguments:
--service-instance-id: PowerVS service instance id.

--zone: PowerVS service instance zone.

--network: Network ID or Name.

#### Example:
```shell
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs network delete --network <network-name/network-id> --service-instance-id <service-instance-id> --zone <zone>
```


### 3. capibmadm powervs network list

#### Usage:
List PowerVS networks.

#### Environmental Variable:
IBMCLOUD_API_KEY: IBM Cloud API key.

#### Arguments:
--service-instance-id: PowerVS service instance id.

--zone: PowerVS service instance zone.

#### Example:
```shell
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs network list --service-instance-id <service-instance-id> --zone <zone>
```
