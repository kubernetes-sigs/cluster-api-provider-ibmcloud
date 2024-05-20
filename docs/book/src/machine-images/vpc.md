# VPC Images


| Region   | Bucket           | Object                                                   | Kubernetes Version |
|----------|------------------|----------------------------------------------------------|--------------------|
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2204-kube-v1-29-3.qcow2][kube-1-29-3] | 1.29.3             |
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2004-kube-v1-28-4.qcow2][kube-1-28-4] | 1.28.4             |
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2004-kube-v1-27-2.qcow2][kube-1-27-2] | 1.27.2             |
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2004-kube-v1-26-2.qcow2][kube-1-26-2] | 1.26.2             |
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2004-kube-v1-25-6.qcow2][kube-1-25-6] | 1.25.6             |
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2004-kube-v1-25-2.qcow2][kube-1-25-2] | 1.25.2             |

Note: These images are built using the [image-builder][image-builder] tool and more information can be found [here](../developer/build-images.md#vpc)

[kube-1-29-3]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2204-kube-v1-29-3.qcow2
[kube-1-28-4]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2004-kube-v1-28-4.qcow2
[kube-1-27-2]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2004-kube-v1-27-2.qcow2
[kube-1-26-2]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2004-kube-v1-26-2.qcow2
[kube-1-25-6]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2004-kube-v1-25-6.qcow2
[kube-1-25-2]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2004-kube-v1-25-2.qcow2

[image-builder]: https://github.com/kubernetes-sigs/image-builder
