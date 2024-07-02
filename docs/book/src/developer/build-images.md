# How to build the machine boot images

## VPC

- [Building CAPI Images for IBMCLOUD (CAPIBM) VPC](https://image-builder.sigs.k8s.io/capi/providers/ibmcloud.html#capibm---vpc)

### Example

To build an image using a specific version of Kubernetes use the "PACKER_FLAGS" environment variable like in the example below:

```shell
# Clone the image-builder repository
$ git clone https://github.com/kubernetes-sigs/image-builder.git
$ cd image-builder/images/capi
$ PACKER_FLAGS="--var 'kubernetes_rpm_version=1.26.2-0' --var 'kubernetes_semver=v1.26.2' --var 'kubernetes_series=v1.26' --var 'kubernetes_deb_version=1.26.2-00'" make build-qemu-ubuntu-2004
```

## PowerVS

- [Building CAPI Images for IBMCLOUD (CAPIBM) PowerVS](https://image-builder.sigs.k8s.io/capi/providers/ibmcloud.html#capibm---powervs)

### Example

Compose the `user-variables.json` file containing the information for the PowerVS

```json
{
  "account_id": "",
  "apikey": "",
  "capture_cos_access_key": "",
  "capture_cos_bucket": "",
  "capture_cos_region": "",
  "capture_cos_secret_key": "",
  "key_pair_name": "",
  "kubernetes_deb_version": "",
  "kubernetes_rpm_version": "",
  "kubernetes_semver": "",
  "kubernetes_series": "",
  "region": "",
  "service_instance_id": "",
  "ssh_private_key_file": "",
  "zone": "",
  "dhcp_network": "false"
}
```

- `account_id`: IBM Cloud account ID
- `apikey`: IBM Cloud API Key
- `capture_cos_access_key`: IBM Cloud Object Storage(COS) access key where the image will be exported
- `capture_cos_bucket`: IBM Cloud Object Storage(COS) bucket name
- `capture_cos_region`: IBM Cloud Object Storage(COS) bucket region
- `capture_cos_secret_key`: IBM Cloud Object Storage(COS) secret key
- `key_pair_name`: SSH key name present in the PowerVS
- `kubernetes_deb_version`: Kubernetes deb version, e.g: 1.24.2-00
- `kubernetes_rpm_version`: Kubernetes RPM package version, e.g: 1.24.2-0
- `kubernetes_semver`: e.g: v1.24.2
- `kubernetes_series`: e.g: v1.24
- `region`: PowerVS region, e.g: osa
- `service_instance_id`: PowerVS service instance ID
- `ssh_private_key_file`: Path to the SSH private key file used to connect to the vm while image preparation, e.g: /Users/manjunath/.ssh/id_rsa
- `zone`: PowerVS zone, e.g: osa21
- `dhcp_network`: Set to `true` if the image has to be built with DHCP support

> **Note:**
> 1. When setting `dhcp_network: true`, you need to build an OS image with certain network settings using [pvsadm tool](https://github.com/ppc64le-cloud/pvsadm/blob/main/docs/Build%20DHCP%20enabled%20Centos%20Images.md) and replace [the fields](https://github.com/kubernetes-sigs/image-builder/blob/cb925047f388090a0db3430ca3172da63eff952c/images/capi/packer/powervs/centos-8.json#L6) with the custom image details.
> 2. Clone the image-builder repo and run `make build` commands from a system where the DHCP private IP can be reached and SSH able(you can use a transit gateway with connections added for VPC and PowerVS workspace and build the image from a virtual server instance in VPC).


```shell
# Clone the image-builder repository
$ git clone https://github.com/kubernetes-sigs/image-builder.git
$ cd image-builder/images/capi
$ ANSIBLE_SSH_ARGS="-o HostKeyAlgorithms=+ssh-rsa -o PubkeyAcceptedAlgorithms=+ssh-rsa" PACKER_VAR_FILES=user-variables.json make build-powervs-centos-8
```
