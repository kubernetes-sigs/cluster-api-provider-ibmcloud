# capibmadm CLI

Kubernetes Cluster API Provider IBM Cloud Management Utility

## Install capibmadm

{{#tabs name:"install-ccapibmadm" tabs:"Linux/MacOS,Windows"}}
{{#tab Linux/MacOS}}

#### Install capibmadm binary with curl on Linux / MacOS
Run the following command to download the capibmadm binary:

```bash
curl -L "https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases/download/v0.11.0/capibmadm-$(echo "$(uname -s)" | tr A-Z a-z)-$(uname -m)" -o capibmadm
```
Add the execute bit to the binary.
```bash
chmod +x ./capibmadm
```
Move the binary to $PATH.
```bash
sudo mv ./capibmadm /usr/local/bin/capibmadm
```
Test to ensure the version you installed is up-to-date:
```bash
capibmadm version -o short
```

{{#/tab }}
{{#tab Windows}}

#### Install capibmadm binary with curl on Windows using PowerShell
Go to the working directory where you want capibmadm downloaded.

Download the latest release on AMD64; on Windows, type:
```powershell
curl.exe -L https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases/download/v0.11.0/capibmadm-windows-amd64.exe -o capibmadm.exe
```
Append or prepend the path of that directory to the `PATH` environment variable.

Download the latest release on ARM64; on Windows, type:
```powershell
curl.exe -L https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases/download/v0.11.0/capibmadm-windows-arm64.exe -o capibmadm.exe
```
Append or prepend the path of that directory to the `PATH` environment variable.

Test to ensure the version you installed is up-to-date:
```powershell
capibmadm.exe version -o short
```

{{#/tab }}
{{#/tabs }}

## [1. PowerVS commands](./powervs/index.md)
## [2. VPC commands](./vpc/index.md)
