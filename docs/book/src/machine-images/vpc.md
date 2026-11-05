# VPC Images


| Region   | Bucket           | Object                                                   | Kubernetes Version |
|----------|------------------|----------------------------------------------------------|--------------------|
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2404-kube-v1-35-9.qcow2][kube-1-35-9] | 1.35.9             |
| us-south | power-oss-bucket | [capibm-vpc-ubuntu-2404-kube-v1-34-2.qcow2][kube-1-34-2] | 1.34.2             |

Note: These images are built using the [image-builder][image-builder] tool and more information can be found [here](../developer/build-images.md#vpc)

[kube-1-35-9]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2404-kube-v1-35-9.qcow2
[kube-1-34-2]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-vpc-ubuntu-2404-kube-v1.34.2.qcow2

[image-builder]: https://github.com/kubernetes-sigs/image-builder
