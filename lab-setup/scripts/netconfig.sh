cat > /etc/netplan/99_$(hostname)_config.yaml <<EOF
network:
  version: 2
  renderer: networkd
  ethernets:
    enp0s8:
      addresses:
        - $(grep $(hostname) /vagrant/scripts/hostips.sh | awk '{print $1}')/24
EOF

netplan apply