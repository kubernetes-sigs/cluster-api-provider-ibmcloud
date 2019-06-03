#!/usr/bin/env bash
set -e
set -x
(
KUBELET_VERSION={{ .Machine.Spec.Versions.Kubelet }}
TOKEN={{ .Token }}
MASTER={{ call .GetMasterEndpoint }}
#NAMESPACE={{ .Machine.ObjectMeta.Namespace }}
NAMESPACE=ibmcloud
MACHINE=$NAMESPACE
MACHINE+="/"
MACHINE+={{ .Machine.ObjectMeta.Name }}
CLUSTER_DNS_DOMAIN={{ .Cluster.Spec.ClusterNetwork.ServiceDomain }}
POD_CIDR={{ .PodCIDR }}
SERVICE_CIDR={{ .ServiceCIDR }}
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

swapoff -a
# disable swap in fstab
sed -i.bak -r 's/(.+ swap .+)/#\1/' /etc/fstab

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


# Set up kubeadm config file to pass to kubeadm join.
cat > /etc/kubernetes/kubeadm_config.yaml <<EOF
apiVersion: kubeadm.k8s.io/v1beta1
kind: JoinConfiguration
caCertPath: /etc/kubernetes/pki/ca.crt
discovery:
  bootstrapToken:
    apiServerEndpoint: ${MASTER}
    token: ${TOKEN}
    unsafeSkipCAVerification: true
  timeout: 5m0s
  tlsBootstrapToken: ${TOKEN}
nodeRegistration:
  criSocket: /var/run/dockershim.sock
EOF

systemctl enable kubelet.service

modprobe br_netfilter
echo '1' > /proc/sys/net/bridge/bridge-nf-call-iptables
echo '1' > /proc/sys/net/ipv4/ip_forward

kubeadm join -v 10 --ignore-preflight-errors=all --config /etc/kubernetes/kubeadm_config.yaml
for tries in $(seq 1 60); do
	kubectl --kubeconfig /etc/kubernetes/kubelet.conf annotate --overwrite node $(hostname) machine=${MACHINE} && break
	sleep 1
done
echo done.
) 2>&1 | tee /var/log/startup.log
