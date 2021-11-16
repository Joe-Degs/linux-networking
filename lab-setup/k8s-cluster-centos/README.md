# Kubernetes the Hard Way
 What is the easiest way of setting up a kubernetes cluster?
 - let somebody else do it for you.

In the spirit of letting somebody else(me being the somebody else) do it for us. 
Lets dive into kubernetes and figure out how it works. I an adapted [kubernetes-the-hard-way](https://github.com/sgargel/kubernetes-the-hard-way) for virtual box.

This is going to be a three node cluster
* control plane -> 192.168.1.1 server0
* worker node 1 -> 192.168.1.100 server1
* worker node 2 -> 192.168.1.200 server2

The machines were provisioned with `vagrant`, with this [Vagrantfile](https://github.com/Joe-Degs/linux-networking/tree/lab-setup/Vagrantfile)
The machines can locate each other on a local network and talk without problems.
