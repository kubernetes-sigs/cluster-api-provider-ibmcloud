# Kubernetes Cluster API Provider IBM Cloud

<p align="center">
<img src="../images/k8s-ibm-cloud.png" alt="Kubernetes Cluster API Provider IBM Cloud">
</p>

------
<p align="center">Kubernetes-native declarative infrastructure for IBM Cloud.</p>

## What is the Cluster API Provider IBM Cloud

The [Cluster API](https://github.com/kubernetes-sigs/cluster-api) brings declarative, Kubernetes-style APIs to cluster creation, configuration and management.

The API itself is shared across multiple cloud providers allowing for true IBM Cloud
hybrid deployments of Kubernetes.  It is built atop the lessons learned from
previous cluster managers such as [kops](https://github.com/kubernetes/kops) and
[kubicorn](http://kubicorn.io/).

<aside class="note">

<h1>Cluster API Provider IBM Cloud documentation versions</h1>

This book documents Cluster API Provider IBM Cloud v0.6. For other versions please see the corresponding documentation:
* [main.cluster-api-ibmcloud.sigs.k8s.io](https://main.cluster-api-ibmcloud.sigs.k8s.io)
* [release-0-6.cluster-api-ibmcloud.sigs.k8s.io](https://release-0-6.cluster-api-ibmcloud.sigs.k8s.io/)
* [release-0-5.cluster-api-ibmcloud.sigs.k8s.io](https://release-0-5.cluster-api-ibmcloud.sigs.k8s.io/)
* [release-0-4.cluster-api-ibmcloud.sigs.k8s.io](https://release-0-4.cluster-api-ibmcloud.sigs.k8s.io/)

</aside>

## CAPIBM Supported Infrastructure-as-a-Service (IaaS)

<p align="center">
<img src="../images/ibm-cloud-iaas.png" alt="Supported IBM Cloud IaaS">
</p>

Currently, the CAPIBM project exclusively facilitates the deployment of Kubernetes (K8s) clusters solely on two IBM infrastructure offerings, namely [IBM VPC (Virtual Server Instances)](https://cloud.ibm.com/docs/vpc?topic=vpc-about-advanced-virtual-servers) and [IBM PowerVS](https://cloud.ibm.com/docs/power-iaas?topic=power-iaas-about-virtual-server).

## Quick Start

Check out the [getting started](./getting-started.html) section to create your first Kubernetes cluster on IBM Cloud using Cluster API.

## Tilt-based development environment

See [developer guide](/developer/tilt.html) section for details.


## Documentation

Please see our [Book](https://cluster-api-ibmcloud.sigs.k8s.io) for in-depth user documentation.

Additional docs can be found in the `/docs` directory, and the [index is here](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/tree/main/docs).

## Getting involved and contributing

Are you interested in contributing to cluster-api-provider-ibmcloud? We, the
maintainers and community, would love your suggestions, contributions, and help!
Also, the maintainers can be contacted at any time to learn more about how to get
involved.

In the interest of getting more new people involved, we tag issues with
[`good first issue`](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/issues?q=is%3Aissue+label%3A%22good+first+issue%22+is%3Aopen).
These are typically issues that have smaller scope but are good ways to start
to get acquainted with the codebase.

We also encourage all active community participants to act as if they are
maintainers, even if you don't have "official" write permissions. This is a
community effort, we are here to serve the Kubernetes community. If you have an
active interest and you want to get involved, you have real power! Don't assume
that the only people who can get things done around here are the "maintainers".

We also would love to add more "official" maintainers, so show us what you can
do!

This repository uses the Kubernetes bots.  See a full list of the commands [here](https://prow.k8s.io/command-help).

### Join us

The community holds a bi-weekly meeting every Friday at 09:00 (IST) / 03:30 (UTC) on [Zoom](https://zoom.us/j/9392903494). Subscribe to the [SIG Cluster Lifecycle](https://groups.google.com/g/kubernetes-sig-cluster-lifecycle) Google Group for access to documents and calendars


### Other ways to communicate with the contributors

Please check in with us in the [#cluster-api-ibmcloud](https://kubernetes.slack.com/archives/C02F4CX3ALF) channel on Slack.

## Github issues

### Bugs

If you think you have found a bug please follow the instructions below.

- Please spend a small amount of time giving due diligence to the issue tracker. Your issue might be a duplicate.
- Get the logs from the cluster controllers. Please paste this into your issue.
- Open a [bug report](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/issues/new?assignees=&labels=&template=bug_report.md).
- Remember users might be searching for your issue in the future, so please give it a meaningful title to helps others.

### Tracking new features

We also use the issue tracker to track features. If you have an idea for a feature, or think you can help Cluster API Provider IBMCloud become even more awesome, then follow the steps below.

- Open a [feature request](https://github.com/kubernetes-sigs/cluster-api-provider-ibmcloud/issues/new?assignees=&labels=&template=feature_request.md).
- Remember users might be searching for your issue in the future, so please
  give it a meaningful title to helps others.
- Clearly define the use case, using concrete examples. EG: I type `this` and
  cluster-api-provider-ibmcloud does `that`.
- Some of our larger features will require some design. If you would like to
  include a technical design for your feature please include it in the issue.
- After the new feature is well understood, and the design agreed upon we can
  start coding the feature. We would love for you to code it. So please open
  up a **WIP** *(work in progress)* pull request, and happy coding.