#!/bin/bash

# run this script as root

# takes a hostname in the cluster and returns its ipaddress
get_ip(){
    grep $1 $SHARED_DIR/scripts/hostips.sh | awk '{print $1}'
}

SHARED_DIR="/vagrant"
CERTS_DIR="$SHARED_DIR/certs"

# make certs on one machine and copy to the rest
create_certs() {
    mkdir -p $CERTS_DIR && cd $CERTS_DIR
    
    cat > ca-config.json <<EOF
{
    "signing": {
        "default": {
        "expiry": "8760h"
        },
        "profiles": {
            "kubernetes": {
                "usages": ["signing", "key encipherment", "server auth", "client auth"],
                "expiry": "8760h"
            }
        }
    }
}
EOF

    cat > ca-csr.json <<EOF
{
    "CN": "Kubernetes",
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "names": [
        {
            "C": "IE",
            "L": "Cork",
            "O": "Kubernetes",
            "OU": "CA",
            "ST": "Cork Co."
        }
    ]
}
EOF

    # generate certificate authority and perm file
    cfssl gencert -initca ca-csr.json | cfssljson -bare ca

    # certificate signing request file
    cat > kubernetes-csr.json <<EOF
{
    "CN": "kubernetes",
    "key": {
        "algo": "rsa",
        "size": 2048
    },
    "names": [
        {
            "C": "IE",
            "L": "Cork",
            "O": "Kubernetes",
            "OU": "Kubernetes",
            "ST": "Cork Co."
        }
    ]
}
EOF
    cfssl gencert \
        -ca=ca.pem \
        -ca-key=ca-key.pem \
        -config=ca-config.json \
        -hostname=$(get_ip master0),$(get_ip master1),$(get_ip master2),127.0.0.1,kubernetes.default \
        -profile=kubernetes kubernetes-csr.json | \
        cfssljson -bare kubernetes
    
    cd ~
}

[[ "$(hostname)" == "master1" ]] && create_certs

# configuring etcd on all controller nodes
cd ~
mkdir -p /etc/etcd /var/lib/etcd
#wget https://github.com/etcd-io/etcd/releases/download/v3.3.13/etcd-v3.3.13-linux-amd64.tar.gz
tar xvzf $SHARED_DIR/download/etcd-v3.3.13-linux-amd64.tar.gz
mv etcd-v3.3.13-linux-amd64/etcd* /usr/local/bin/
cd $CERTS_DIR && cp *.pem /etc/etcd/

IPA="$(get_ip master0)"
IPB="$(get_ip master1)"
IPC="$(get_ip master2)"
HOST_IP="$(get_ip $(hostname))"


cat <<EOF > /etc/systemd/system/etcd.service
[Unit]
Description=etcd
Documentation=https://github.com/coreos

[Service]
ExecStart=/usr/local/bin/etcd \
  --name $HOST_IP \
  --key-file=/etc/etcd/kubernetes-key.pem \
  --peer-cert-file=/etc/etcd/kubernetes.pem \
  --peer-key-file=/etc/etcd/kubernetes-key.pem \
  --trusted-ca-file=/etc/etcd/ca.pem \
  --peer-trusted-ca-file=/etc/etcd/ca.pem \
  --peer-client-cert-auth \
  --client-cert-auth \
  --initial-advertise-peer-urls https://$HOST_IP:2380 \
  --listen-peer-urls https://0.0.0.0:2380 \
  --listen-client-urls https://$HOST_IP:2379,http://127.0.0.1:2379 \
  --advertise-client-urls https://$HOST_IP:2379 \
  --initial-cluster-token etcd-cluster-0 \
  --initial-cluster $IPA=https://$IPA:2380,$IPB=https://$IPB:2380,$IPC=https://$IPC:2380 \
  --initial-cluster-state new \
  --data-dir=/var/lib/etcd
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF

fuser -k 2380/tcp
fuser -k 2379/tcp
systemctl daemon-reload
systemctl enable etcd
systemctl start etcd