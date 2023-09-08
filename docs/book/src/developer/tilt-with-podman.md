# Podman setup to use tilt


## Prerequisites

1. Install Podman: Instruction can be found [here](https://podman.io/docs/installation)
2. Emulate docker cli with Podman: Instructions can be found [here](https://podman-desktop.io/docs/migrating-from-docker/emulating-docker-cli-with-podman)

## 1. Create Podman machine

```shell
$ podman machine init
$ podman machine start
```

## 2. Configure podman to use local registry

```shell
$ podman machine ssh
$ sudo vi /etc/containers/registries.conf

## at the end of the file add below content

[[registry]]
location = "localhost:5001"
insecure = true
```
Restart Podman machine

```shell
podman machine stop
podman machine start
```

## 3. Create a kind cluster

```shell
$ make kind-cluster
```