#!/bin/bash

cat >> /etc/hosts <<EOF
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