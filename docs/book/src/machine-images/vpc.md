# VPC Images


| Region   | Bucket           | Object                                                   | Kubernetes Version |
|----------|------------------|----------------------------------------------------------|--------------------|
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2404-kube-v1-33-0.qcow2][kube-1-33-0] | 1.33.0             |
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2404-kube-v1-32-3.qcow2][kube-1-32-3] | 1.32.3             |
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2204-kube-v1-31-4.qcow2][kube-1-31-4] | 1.31.4             |
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2204-kube-v1-30-4.qcow2][kube-1-30-4] | 1.30.4             |

Note: These images are built using the [image-builder][image-builder] tool and more information can be found [here](../developer/build-images.md#vpc)

[kube-1-33-0]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/ubuntu-2404-kube-v1.33.0.qcow2
[kube-1-32-3]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/ubuntu-2404-kube-v1.32.3.qcow2
[kube-1-31-4]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2204-kube-v1-31-4.qcow2
[kube-1-30-4]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2204-kube-v1-30-4.qcow2

[image-builder]: https://github.com/kubernetes-sigs/image-builder
