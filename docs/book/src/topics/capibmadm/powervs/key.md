## PowerVS SSH key Commands

### 1. capibmadm powervs key create

#### Usage:
Create an SSH key in the PowerVS environment.

#### Environmental Variable:
IBMCLOUD_API_KEY: IBM Cloud API key.

#### Arguments:
--service-instance-id: PowerVS service instance id.

--zone: PowerVS zone.

--name: The name of the SSH key.

Either of the arguments need to be provided:

--key: SSH RSA key string within a double quotation marks. For example, "ssh-rsa AAA... ".

--key-path: The absolute path to the SSH key file.

#### Example:
```shell
export IBMCLOUD_API_KEY=<api-key>

# Using SSH key
capibmadm powervs key create --name <key-name> --key "<ssh-key>" --service-instance-id <service-instance-id> --zone <zone>

# Using file-path to SSH key
capibmadm powervs key create --name <key-name> --key-path <path/to/ssh/key> --service-instance-id <service-instance-id> --zone <zone>
```

### 2. capibmadm powervs key delete

#### Usage:
Delete an SSH key in the PowerVS environment.

#### Environmental Variable:
IBMCLOUD_API_KEY: IBM Cloud API key.

#### Arguments:
--service-instance-id: PowerVS service instance id.

--zone: PowerVS zone.

--name: The name of the SSH key.

#### Example:
```shell
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs key delete --name <key-name> --service-instance-id <service-instance-id> --zone <zone>
```

### 3. capibmadm powervs key list

#### Usage:
List all SSH Keys in the PowerVS environment.

#### Environmental Variable:
IBMCLOUD_API_KEY: IBM Cloud API key.

#### Arguments:
--service-instance-id: PowerVS service instance id.

--zone: PowerVS zone.

#### Example:
```shell
export IBMCLOUD_API_KEY=<api-key>
capibmadm powervs key list --service-instance-id <service-instance-id> --zone <zone>
```
