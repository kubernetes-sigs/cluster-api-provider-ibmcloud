# capibmadm CLI

Kubernetes Cluster API Provider IBM Cloud Management Utility

## Install capibmadm

{{#tabs name:"install-capibmadm" tabs:"Linux,macOS,Windows"}}
{{#tab Linux}}

#### Install capibmadm binary with curl on Linux
If you are unsure you can determine your computers architecture by running `uname -a`

Download for AMD64:
```bash
curl -L {{#releaselink repo:"https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud" gomodule:"sigs.k8s.io/cluster-api-provider-ibmcloud" asset:"capibmadm-linux-amd64" version:"0.12.x"}} -o capibmadm
```

Download for ARM64:
```bash
curl -L {{#releaselink repo:"https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud" gomodule:"sigs.k8s.io/cluster-api-provider-ibmcloud" asset:"capibmadm-linux-arm64" version:"0.12.x"}} -o capibmadm
```

Download for PPC64LE:
```bash
curl -L {{#releaselink repo:"https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud" gomodule:"sigs.k8s.io/cluster-api-provider-ibmcloud" asset:"capibmadm-linux-ppc64le" version:"0.12.x"}} -o capibmadm
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
{{#tab macOS}}

#### Install capibmadm binary with curl on MacOS
If you are unsure you can determine your computers architecture by running `uname -a`

Download for AMD64:
```bash
curl -L {{#releaselink repo:"https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud" gomodule:"sigs.k8s.io/cluster-api-provider-ibmcloud" asset:"capibmadm-darwin-amd64" version:"0.12.x"}} -o capibmadm
```

Download for M1 CPU ("Apple Silicon") / ARM64:
```bash
curl -L {{#releaselink repo:"https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud" gomodule:"sigs.k8s.io/cluster-api-provider-ibmcloud" asset:"capibmadm-darwin-arm64" version:"0.12.x"}} -o capibmadm
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
curl.exe -L {{#releaselink repo:"https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud" gomodule:"sigs.k8s.io/cluster-api-provider-ibmcloud" asset:"capibmadm-windows-amd64.exe" version:"0.12.x"}} -o capibmadm.exe
```
Append or prepend the path of that directory to the `PATH` environment variable.

Download the latest release on ARM64; on Windows, type:
```powershell
curl.exe -L {{#releaselink repo:"https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud" gomodule:"sigs.k8s.io/cluster-api-provider-ibmcloud" asset:"capibmadm-windows-amd64.exe" version:"0.12.x"}} -o capibmadm.exe
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
