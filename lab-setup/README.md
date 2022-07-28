## setting up a kubernetes Cluster on Local machine with virtual box and vagrant
this directory contains work in progress setup scripts and tools for deploying
a kubernetes cluster on localhost.

this whole endeavour is an adaptation of Kelsey Hightowers [kubernetes-the-hardway](https://github.com/kelseyhightower/kubernetes-the-hard-way) walkthrough.

- the `Vagrantfile` in this directory contains machine config and initial setup.

## ./scripts
contains all the scripts for setting up the machines, configuring ip so they are
on the same network and can talk to each other and installing and configuring 
haproxy, etcd and kubernetes.

- the `clusterfuck.go` program is a silly tool for executing scripts concurrently
  with ssh on the guest vms that form the cluster.

- `hostips.sh` is for configuring network hosts in the cluster.
- `netconfig.sh` configures each host in the cluster with its specified ip in the hostips script.
- `install-binaries.sh` installs all the binary files needed in the cluster.
- `gen-certs.sh` generates all the certificates for securing endpoints of apis in the cluster.
- `setup-haproxy.sh` sets up a haproxy load balancer for balancing load from clients to the servers in the cluster.
- `setup-etcd.sh` bootstraps etcd on the controller nodes (`master0-n`)
- `bootstrap-controllers.sh` bootstraps kubernetes on the controller nodes.