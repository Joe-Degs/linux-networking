#!/usr/bin/env bash
cat <<EOF > /etc/yum.repos.d/kubernetes.repo
[kubernetes]
name=Kubernetes
baseurl=https://packages.cloud.google.com/yum/repos/kubernetes-el7-x86_64
enabled=1
gpgcheck=1
repo_gpgcheck=1
gpgkey=https://packages.cloud.google.com/yum/doc/yum-key.gpg https://packages.cloud.google.com/yum/doc/rpm-package-key.gpg
EOF

# install runc and containerd
yum install -y runc
wget https://github.com/containerd/containerd/releases/download/v1.5.7/containerd-1.5.7-linux-amd64.tar.gz
tar xvf containerd-1.5.7-linux-amd64.tar.gz
mv bin/* /usr/local/bin/
rm -rf containerd-1.5.7-linux-amd64.tar.gz bin
mkdir -p /etc/containerd
containerd config default > /etc/containerd/config.toml

# install kubernetes
sudo yum install -y kubelet kubeadm kubectl
systemctl enable kubelet
systemctl start kubelet
