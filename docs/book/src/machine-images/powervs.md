# PowerVS Images


| Region   | Bucket           | Object                                                          | Kubernetes Version |
|----------|------------------|-----------------------------------------------------------------|--------------------|
| us-south | power-oss-bucket | [capibm-powervs-centos-streams9-1-31-0.ova.gz][streams9-1-31-0] | 1.31.0             |
| us-south | power-oss-bucket | [capibm-powervs-centos-streams9-1-30-0.ova.gz][streams9-1-30-0] | 1.30.0             |
| us-south | power-oss-bucket | [capibm-powervs-centos-streams8-1-29-3.ova.gz][streams8-1-29-3] | 1.29.3             |

## PowerVS Images with DHCP based network

| Region   | Bucket           | Object                                                                            | Kubernetes Version |
|----------|------------------|-----------------------------------------------------------------------------------|--------------------|
| us-south | power-oss-bucket | [capibm-powervs-centos-streams9-1-29-3-1719470782.ova.gz][centos-streams9-1-29-3] | 1.29.3             |

> **Note:** These images are built using the [image-builder][image-builder] tool and more information can be found [here](../developer/build-images.md#powervs)

[streams9-1-31-0]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-powervs-centos-streams9-1-31-0-1737533452.ova.gz
[streams9-1-30-0]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-powervs-centos-streams9-1-30-0-1737523124.ova.gz
[centos-streams9-1-29-3]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-powervs-centos-streams9-1-29-3-1719470782.ova.gz
[streams8-1-29-3]: https://power-oss-bucket.s3.us-south.cloud-object-storage.appdomain.cloud/capibm-powervs-centos-streams8-1-29-3.ova.gz

[image-builder]: https://github.com/kubernetes-sigs/image-builder
