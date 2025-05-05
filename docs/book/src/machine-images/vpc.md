# VPC Images


| Region   | Bucket           | Object                                                   | Kubernetes Version |
|----------|------------------|----------------------------------------------------------|--------------------|
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2204-kube-v1-31-4.qcow2][kube-1-31-4] | 1.31.4             |
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2204-kube-v1-29-3.qcow2][kube-1-29-3] | 1.29.3             |

Note: These images are built using the [image-builder][image-builder] tool and more information can be found [here](../developer/build-images.md#vpc)

[kube-1-31-4]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2204-kube-v1-31-4.qcow2
[kube-1-29-3]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2204-kube-v1-29-3.qcow2

[image-builder]: https://github.com/kubernetes-sigs/image-builder
