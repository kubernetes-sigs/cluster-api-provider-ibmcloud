<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->
**Table of Contents**  *generated with [DocToc](https://github.com/thlorenz/doctoc)*

- [IBM Cloud](#ibm-cloud)
  - [How to set correct OS reference code in `machines.yaml`?](#how-to-set-correct-os-reference-code-in-machinesyaml)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

# IBM Cloud
## How to set correct OS reference code in `machines.yaml`?
The value of `osReferenceCode` is from IBM Cloud. You can get an avalible value list by:
```bash
slcli virtual create-options
```
If you're using `ubuntu`, the value of name `os (UBUNTU)` lists all available OS reference code.
You can use the specific OS version for the `Machine` by configurating corresponding OS reference code.

Please refer to the [softlayer command line document](https://softlayer-api-python-client.readthedocs.io/en/latest/cli/)
for details in setting up `slcli`
