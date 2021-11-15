#!/usr/bin/env bash

mkdir ~/k8s && cd ~/k8s
wget -q \
   https://pkg.cfssl.org/R1.2/cfssl_linux-amd64 \
   https://pkg.cfssl.org/R1.2/cfssljson_linux-amd64

chmod +x cfssl_linux-amd64 cfssljson_linux-amd64
sudo mv cfssl_linux-amd64 /usr/local/bin/cfssl
sudo mv cfssljson_linux-amd64 /usr/local/bin/cfssljson

# install kubectl
wget https://storage.googleapis.com/kubernetes-release/release/v1.18.2/bin/linux/amd64/ku
bectl
chmod +x kubectl
sudo mv kubectl /usr/local/bin/
