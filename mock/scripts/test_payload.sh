#!/bin/sh
mount /dev/vdb /workspace

# Configure networking
ip addr add 172.16.0.2/24 dev eth0
ip link set eth0 up
ip route add default via 172.16.0.1
echo 'nameserver 8.8.8.8' > /etc/resolv.conf

# Run the contestant's binary directly
/workspace/orderbook-server