# Uploading an image to the IBM Cloud

Build the Ubuntu image as described in the previous [VPC section](prerequisites.md).
Make sure to build the qcow2 version by following the instructions for [ibmcloud vpc image build](https://image-builder.sigs.k8s.io/capi/providers/ibmcloud.html#capibm---vpc).

Since the IBM Cloud does not support dots before the qcow2 extension, rename the file as follows:
```console
ubuntu-2004-ibmcloud-kube-v1-23-4.qcow2
```

## Upload VM image:

1) Create an IBM COS instance
2) Create a bucket in the COS instance.
3) Upload the image
   1) Upload via aspera
        * Install the browser extension for Aspera
        * Downloading the Aspera tool
        * Selecting the image via Aspera dialog
        * Upload the image via aspera
   2) Using minio cli
        * Install minio cli
        *  Creating a service credential with hmac=true for the bucket
        * Example upload for eu-de:
            ```sh
            mc alias set uploadcos https://s3.eu-de.cloud-object-storage.appdomain.cloud <hmac access id> <hmac secret key>
            ```
            ```sh
            mc cp <image-name>.qcow2 uploadcos/<my-bucket-name>
            ```

## Add VM image to VPC

1) Make sure you have editor rights for all/most VPC services
2) Add additional read rights for:
```console
src: service VPC Infrastructure Services resourceType equals image
target: serviceInstance string equals <your-Cloud-Object Storage-VM-plain-name>
```

Add write rights for:
```console
Service VPC Infrastructure Services in Resource_group <your_resource_group_or_account> resourceType equals image
target: service Cloud object storage in resource_group <your_resource_group_or_account>
```

3) Go to [https://cloud.ibm.com/vpc-ext/provision/customImage](https://cloud.ibm.com/vpc-ext/provision/customImage)
  * Fill in imagename, resource group or account
  * Choice box: Cloud Object Storage
  * Set Filter: <your_cos_plain_name> <eu-de_or_other> <your_vm_bucket>
  * Choice box: Select your image
  * Select base os (ubuntu-20-04-amd64 for example)
  * Click Create Image

Now you can provision a VM with your own VM image.
Then please continue with
[creating a cluster](creating-a-cluster.md).

Make sure you take the ImageID from your VM image. The ImageID can be determined using ibmcloud cli. In addition, the Kubernetes version must be set to match the image. In this example:
```console
v1.23.4
```
