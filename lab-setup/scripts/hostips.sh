#!/bin/bash

cat >> /etc/hosts <<EOF
127.0.0.1       localhost

# The following lines are desirable for IPv6 capable hosts
::1     ip6-localhost   ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
ff02::3 ip6-allhosts
127.0.1.1       ubuntu-bionic   ubuntu-bionic

# cluster ip addresses
192.168.1.1 master0
192.168.1.10 master1
192.168.1.11 master2
192.168.1.111 gateway
192.168.1.100 worker0
192.168.1.101 worker1
192.168.1.102 worker2
192.168.1.103 worker3
192.168.1.104 worker4
192.168.1.105 worker5
EOF