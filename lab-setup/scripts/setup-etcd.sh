#!/bin/bash

# bootstrapp the etcd node

SHARED_DIR="/vagrant"
CERTS_DIR="$SHARED_DIR/certs"

# takes a hostname in the cluster and returns its ipaddress
get_ip(){
    grep $1 $SHARED_DIR/scripts/hostips.sh | awk '{print $1}'
}

# configuring etcd on all controller nodes
if [ ! -e /var/lib/etcd ]; then
  mkdir -p /etc/etcd /var/lib/etcd
  tar xvzf $SHARED_DIR/download/etcd-v3.3.13-linux-amd64.tar.gz
  mv etcd-v3.3.13-linux-amd64/etcd* /usr/local/bin/
  cp $CERTS_DIR/{ca.pem,kubernetes-key.pem,kubernetes.pem} /etc/etcd/
fi

IPA="$(get_ip master0)"
IPB="$(get_ip master1)"
IPC="$(get_ip master2)"
HOST_IP="$(get_ip $(hostname))"


cat <<EOF > /etc/systemd/system/etcd.service
[Unit]
Description=etcd
Documentation=https://github.com/coreos

[Service]
ExecStart=/usr/local/bin/etcd \\
  --name $(hostname) \\
  --cert-file=/etc/etcd/kubernetes.pem \\
  --key-file=/etc/etcd/kubernetes-key.pem \\
  --peer-cert-file=/etc/etcd/kubernetes.pem \\
  --peer-key-file=/etc/etcd/kubernetes-key.pem \\
  --trusted-ca-file=/etc/etcd/ca.pem \\
  --peer-trusted-ca-file=/etc/etcd/ca.pem \\
  --peer-client-cert-auth \\
  --client-cert-auth \\
  --initial-advertise-peer-urls https://${HOST_IP}:2380 \\
  --listen-peer-urls https://${HOST_IP}:2380 \\
  --listen-client-urls https://${HOST_IP}:2379,http://127.0.0.1:2379 \\
  --advertise-client-urls https://${HOST_IP}:2379 \\
  --initial-cluster-token etcd-cluster-0 \\
  --initial-cluster master0=https://${IPA}:2380,master1=https://${IPB}:2380,master2=https://${IPC}:2380 \\
  --initial-cluster-state new \\
  --data-dir=/var/lib/etcd
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

systemctl enable etcd
systemctl restart etcd