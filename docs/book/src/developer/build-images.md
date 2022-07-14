# How to build the machine boot images

## VPC
TO-DO

## Power VS

Compose the `user-varibales.json` file contains the information for the Power VS

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
  "zone": ""
}
```

- `account_id`: IBM Cloud account ID
- `apikey`: IBM Cloud API Key
- `capture_cos_access_key`: Access key for the IBM Cloud Object Storage(COS) to export the image to
- `capture_cos_bucket`: IBM Cloud Object Storage(COS) bucket name
- `capture_cos_region`: IBM Cloud Object Storage(COS) bucket region
- `capture_cos_secret_key`: IBM Cloud Object Storage(COS) secret key
- `key_pair_name`: SSH key name present in the Power VS
- `kubernetes_deb_version`: Kubernetes deb version, e.g: 1.24.2-00
- `kubernetes_rpm_version`: Kubernetes RPM package version, e.g: 1.24.2-0
- `kubernetes_semver`: e.g: v1.24.2
- `kubernetes_series`: e.g: v1.24
- `region`: Power VS region, e.g: osa
- `service_instance_id`: Power VS service instance ID
- `ssh_private_key_file`: Path to the SSH private key file used to connect to the vm while image preparation, e.g: /Users/manjunath/.ssh/id_rsa
- `zone`: Power VS zone, e.g: osa21

```shell
# Clone the image-builder repository
$ git clone https://github.com/kubernetes-sigs/image-builder.git
$ cd image-builder/images/capi
$ PACKER_VAR_FILES=user-variables.json make build-powervs-centos-8
```
