#!/usr/bin/env bash
set -e
set -x
(
KUBELET_VERSION={{ .Machine.Spec.Versions.Kubelet }}
VERSION=v${KUBELET_VERSION}
NAMESPACE={{ .Machine.ObjectMeta.Namespace }}
MACHINE=$NAMESPACE
MACHINE+="/"
MACHINE+={{ .Machine.ObjectMeta.Name }}
CONTROL_PLANE_VERSION={{ .Machine.Spec.Versions.ControlPlane }}
CLUSTER_DNS_DOMAIN={{ .Cluster.Spec.ClusterNetwork.ServiceDomain }}
POD_CIDR={{ .PodCIDR }}
SERVICE_CIDR={{ .ServiceCIDR }}
ARCH=amd64

swapoff -a
# disable swap in fstab
sed -i.bak -r 's/(.+ swap .+)/#\1/' /etc/fstab

# Getting master public ip 
# TODO: is there any general way to get IP address in IBM Cloud?
# e.g. Openstack: curl --fail -s http://169.254.169.254/2009-04-04/meta-data/local-ipv4
echo "trying to get public ipv4 $i / 60"
MASTER=$(ip address show eth1 | grep 'inet '| awk '{print $2}' | cut -f1 -d'/')
if [[ -z "$MASTER" ]]; then
    echo "falling back to localhost"
    MASTER="localhost"
fi
MASTER="${MASTER}:443"

cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
exclude=kube*
EOF

if [[ $(getenforce) != 'Disabled' ]]; then
  setenforce 0
fi

yum install -y kubelet-$KUBELET_VERSION kubeadm-$KUBELET_VERSION kubectl-$KUBELET_VERSION --disableexcludes=kubernetes

function install_configure_docker () {
    # prevent docker from auto-starting
    echo "exit 101" > /usr/sbin/policy-rc.d
    chmod +x /usr/sbin/policy-rc.d
    trap "rm /usr/sbin/policy-rc.d" RETURN
    yum install -y docker
    echo 'OPTIONS="--log-driver=journald --signature-verification=false --iptables=false --ip-masq=false"' >> /etc/sysconfig/docker
    systemctl daemon-reload
    systemctl enable docker
    systemctl start docker
}

install_configure_docker

systemctl enable kubelet.service

modprobe br_netfilter
echo '1' > /proc/sys/net/bridge/bridge-nf-call-iptables
echo '1' > /proc/sys/net/ipv4/ip_forward

# Set up kubeadm config file to pass parameters to kubeadm init.
cat > /etc/kubernetes/kubeadm_config.yaml <<EOF
apiVersion: kubeadm.k8s.io/v1beta1
kind: InitConfiguration
bootstrapTokens:
- groups:
  - system:bootstrappers:kubeadm:default-node-token
  token: ${TOKEN}
  ttl: 24h0m0s
  usages:
  - signing
  - authentication
localAPIEndpoint:
  bindPort: 443
nodeRegistration:
  criSocket: /var/run/dockershim.sock
  taints:
  - effect: NoSchedule
    key: node-role.kubernetes.io/master
---
apiVersion: kubeadm.k8s.io/v1beta1
kind: ClusterConfiguration
kubernetesVersion: v${CONTROL_PLANE_VERSION}
apiServer:
  timeoutForControlPlane: 4m0s
certificatesDir: /etc/kubernetes/pki
clusterName: kubernetes
controlPlaneEndpoint: ${MASTER}
controllerManager:
  extraArgs:
    allocate-node-cidrs: "true"
    cluster-cidr: ${POD_CIDR}
    service-cluster-ip-range: ${SERVICE_CIDR}
dns:
  type: CoreDNS
etcd:
  local:
    dataDir: /var/lib/etcd
imageRepository: k8s.gcr.io
networking:
  dnsDomain: cluster.local
  podSubnet: ""
  serviceSubnet: ${SERVICE_CIDR}
EOF

kubeadm init -v 10 --config /etc/kubernetes/kubeadm_config.yaml
for tries in $(seq 1 60); do
    kubectl --kubeconfig /etc/kubernetes/kubelet.conf annotate --overwrite node $(hostname) machine=${MACHINE} && break
    sleep 1
done

# By default, use calico for container network plugin, should make this configurable.
kubectl --kubeconfig /etc/kubernetes/admin.conf apply -f https://docs.projectcalico.org/v3.5/getting-started/kubernetes/installation/hosted/kubernetes-datastore/calico-networking/1.7/calico.yaml

mkdir -p /root/.kube
cp -i /etc/kubernetes/admin.conf /root/.kube/config
chown $(id -u):$(id -g) /root/.kube/config

echo done.
) 2>&1 | tee /var/log/startup.log