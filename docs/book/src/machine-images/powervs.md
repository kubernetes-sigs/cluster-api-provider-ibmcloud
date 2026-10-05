# PowerVS Images

CAPIBM requires a machine boot image to be available in your PowerVS workspace. Two image types are available depending on your [cluster topology](../topics/powervs/index.md):

| Image type | Used with |
|---|---|
| **Standard** images | [VirtualIP topology](../topics/powervs/index.md#option-1--virtualip) — imported manually into an existing workspace |
| **DHCP-enabled** images | [LoadBalancer topology](../topics/powervs/index.md#option-2--loadbalancer) — imported automatically by CAPIBM from COS during cluster creation |

> Images are built using the [image-builder](https://github.com/kubernetes-sigs/image-builder) tool. See [How to build machine boot images](../developer/build-images.md#powervs) for details.

---

## Standard images

| Region | Bucket | Object | Kubernetes Version |
|--------|--------|--------|--------------------|
| us-south | power-oss-bucket | [capibm-powervs-centos-streams10-1-35-9.ova.gz][streams10-1-35-9] | 1.35.9 |
| us-south | power-oss-bucket | [capibm-powervs-centos-streams10-1-34-7.ova.gz][streams10-1-34-7] | 1.34.7 |

To import a standard image manually into your workspace, see [capibmadm powervs image import](../topics/capibmadm/powervs/image.md#1-capibmadm-powervs-image-import).

---

## DHCP-enabled images

These images include a built-in DHCP client configuration required by the [LoadBalancer topology](../topics/powervs/index.md#option-2--loadbalancer), where the network is provisioned by CAPIBM with a DHCP server.

| Region | Bucket | Object | Kubernetes Version |
|--------|--------|--------|--------------------|
| us-south | power-oss-bucket | [capibm-powervs-centos-streams10-1-35-9-dhcp.ova.gz][centos-streams10-1-35-9-dhcp] | 1.35.9 |
| us-south | power-oss-bucket | [capibm-powervs-centos-streams10-1-34-7-dhcp.ova.gz][centos-streams10-1-34-7-dhcp] | 1.34.7 |

Set `COS_BUCKET_NAME`, `COS_BUCKET_REGION`, and `COS_OBJECT_NAME` to the values from this table when using `--flavor=powervs-create-infra`.

---

[streams10-1-35-9]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-powervs-centos-streams10-1-35-9-150500-1-1-1791128145.ova.gz
[streams10-1-34-7]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-powervs-centos-streams10-1-34-7-150500-1-1-1778144615.ova.gz
[centos-streams10-1-35-9-dhcp]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-powervs-centos-streams10-1-35-9-dhcp.ova.gz
[centos-streams10-1-34-7-dhcp]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-powervs-centos-streams10-1-34-7-dhcp.ova.gz

[image-builder]: https://github.com/kubernetes-sigs/image-builder
