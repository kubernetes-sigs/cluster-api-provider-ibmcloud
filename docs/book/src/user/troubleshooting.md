# Troubleshooting

### 1. Tilt stops working as not able to connect to kind cluster

   ```
   % kind get clusters
    enabling experimental podman provider
    ERROR: failed to list clusters: command "podman ps -a --filter label=io.x-k8s.kind.cluster --format '{{index .Labels "io.x-k8s.kind.cluster"}}'" failed with error: exit status 125
    Command Output: Cannot connect to Podman. Please verify your connection to the Linux system using `podman system connection list`, or try `podman machine init` and `podman machine start` to manage a new Linux VM
    Error: unable to connect to Podman socket: failed to connect: dial tcp 127.0.0.1:61514: connect: connection refused
   ```

1. Stop and start the Podman either via cli or from Podman Desktop.
   ```shell
   $ podman machine stop
   $ podman machine start
   ```
2. Run all the stopped containers like capi-test-control-plane, capi-test-worker, kind-registry.
   ```shell
   $ podman container list -a
     CONTAINER ID  IMAGE                                    NAMES
     512cee59230c  docker.io/library/registry:2             kind-registry
     5b99fd84c41e  docker.io/kindest/node@sha256            capi-test-worker
     94130af58929  docker.io/kindest/node@sha256            capi-test-control-plane

   $ podman container start 512cee59230c 5b99fd84c41e 94130af58929
   ```
3. Try re-running `tilt up` from `cluster-api` directory.


### 2. SSH into data/control plane node configured with DHCP network
1. Since the VM backing the node is configured with DHCP network which is private we can't directly SSH into it.
2. Create a public VM in the same workspace and attach the DHCP network to it.

   1. Create public network in PowerVS workspace if it does not exist using ibmcloud cli
   ```shell
   $ibmcloud pi subnet create publicnet1 --net-type public
   ```
   2. List the available images to create VM
   ```shell
   $ibmcloud pi image lc
   ```
   3. Create the VM with public and DHCP subnet.
   ```shell
   $ibmcloud pi instance create publicVM --image testrhel88 --subnets DHCPSERVERcapi-powervs-new_Private,publicnet1
   ```
   4. Get the public IP of created VM
   ```shell
   $ibmcloud pi ins get publicVM
   ```
4. SSH into the DHCP VM using public VM as a jump host.
    ```
   ssh -J root@<public_ip> root@<dhcp_ip>
   ```
