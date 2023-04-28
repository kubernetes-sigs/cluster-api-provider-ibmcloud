# capibmadm CLI

Kubernetes Cluster API Provider IBM Cloud Management Utility

## Install capibmadm

#### Install capibmadm binary with curl on Linux
If you are unsure you can determine your computers architecture by running `uname -a`

Download for AMD64:
```bash
curl -L https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases/download/v0.5.0-alpha.1/capibmadm-linux-amd64 -o capibmadm
```

Download for ARM64:
```bash
curl -L https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases/download/v0.5.0-alpha.1/capibmadm-linux-arm64 -o capibmadm
```

Download for PPC64LE:
```bash
curl -L https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases/download/v0.5.0-alpha.1/capibmadm-linux-ppc64le -o capibmadm
```

Make the capibmadm binary executable.
```bash
chmod +x ./capibmadm
```
Move the binary in to your PATH.
```bash
sudo mv ./capibmadm /usr/local/bin/capibmadm
```
Test to ensure the version you installed is up-to-date:
```bash
capibmadm version -o short
```

#### Install capibmadm binary with curl on macOS
If you are unsure you can determine your computers architecture by running `uname -a`

Download for AMD64:
```bash
curl -L https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases/download/v0.5.0-alpha.1/capibmadm-darwin-amd64 -o capibmadm
```

Download for M1 CPU ("Apple Silicon") / ARM64:
```bash
curl -L https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases/download/v0.5.0-alpha.1/capibmadm-darwin-arm64 -o capibmadm
```

Make the capibmadm binary executable.
```bash
chmod +x ./capibmadm
```
Move the binary in to your PATH.
```bash
sudo mv ./capibmadm /usr/local/bin/capibmadm
```
Test to ensure the version you installed is up-to-date:
```bash
capibmadm version -o short
```

#### Install capibmadm binary with curl on Windows using PowerShell
Go to the working directory where you want capibmadm downloaded.

Download the latest release on AMD64; on Windows, type:
```powershell
curl.exe -L https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases/download/v0.5.0-alpha.1/capibmadm-windows-amd64.exe -o capibmadm.exe
```
Append or prepend the path of that directory to the `PATH` environment variable.

Download the latest release on ARM64; on Windows, type:
```powershell
curl.exe -L https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/releases/download/v0.5.0-alpha.1/capibmadm-windows-arm64.exe -o capibmadm.exe
```
Append or prepend the path of that directory to the `PATH` environment variable.

Test to ensure the version you installed is up-to-date:
```powershell
capibmadm.exe version -o short
```

## [1. PowerVS commands](./powervs/index.md)
## [2. VPC commands](./vpc/index.md)
