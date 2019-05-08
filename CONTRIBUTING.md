# Contributing guidelines

## Sign the CLA

Kubernetes projects require that you sign a Contributor License Agreement (CLA) before we can accept your pull requests.  Please see https://git.k8s.io/community/CLA.md for more info

## Contributing A Patch

1. Submit an issue describing your proposed change to the repo in question.
1. The [repo owners](OWNERS) will respond to your issue promptly.
1. If your proposed change is accepted, and you haven't already done so, sign a Contributor License Agreement (see details above).
1. Fork the desired repo, develop and test your code changes.
1. Submit a pull request.

## Pre-check before submitting a PR

After your PR is ready to commit, please run following commands to check your code:

```shell
make check
make test
```

## Build and push images
Make sure your code build passed:

```shell
export REGISTRY=<your-docker-registry>
make build-push-images
```

## Start kind and deploy the provider

```shell
go get -u sigs.k8s.io/kind && kind create cluster
export KUBECONFIG="$(kind get kubeconfig-path --name="kind")"
make install
make deploy
```

## Verify deployment

TODO: Should deploy a sample Cluster and Machine resource to verify controllers are reconciling properly. This is troublesome however since before the actuator stubs are filled in, all we will see is messages to the effect of "TODO: Not yet implemented"..."

```
kubectl logs cluster-api-provider-ibmcloud-controller-manager-0 -n cluster-api-provider-ibmcloud-system
```