#!/bin/bash

DIR="/vagrant/download"
mkdir -p $DIR && cd $DIR

wget -q --show-progress --https-only --timestamping \
    "https://storage.googleapis.com/kubernetes-release/release/v1.23.5/bin/linux/amd64/kube-apiserver" \
    "https://storage.googleapis.com/kubernetes-release/release/v1.23.5/bin/linux/amd64/kube-controller-manager" \
    "https://storage.googleapis.com/kubernetes-release/release/v1.23.5/bin/linux/amd64/kube-scheduler" \
    "https://storage.googleapis.com/kubernetes-release/release/v1.23.5/bin/linux/amd64/kubectl" \
    "https://github.com/etcd-io/etcd/releases/download/v3.3.13/etcd-v3.3.13-linux-amd64.tar.gz"