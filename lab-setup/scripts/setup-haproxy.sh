#!/bin/bash

## setup haproxy on master one to serve as gateway to the other
## master machines in the cluster and load balance traffic between them

# takes a hostname in the cluster and returns its ipaddress
SHARED_DIR="/vagrant"
get_ip(){
    grep $1 $SHARED_DIR/scripts/hostips.sh | awk '{print $1}'
}

sudo apt-get install -y haproxy 
cat >> /etc/haproxy/haproxy.cfg <<EOF
frontend kubernetes
bind 0.0.0.0:6443
option tcplog
mode tcp
default_backend kubernetes-master-nodes

backend kubernetes-master-nodes
mode tcp
balance roundrobin
option tcp-check
server master0 $(get_ip "master0"):6443 check fall 3 rise 2
server master1 $(get_ip "master1"):6443 check fall 3 rise 2
server master2 $(get_ip "master2"):6443 check fall 3 rise 2
EOF

systemctl restart haproxy