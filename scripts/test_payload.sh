  ``#!/bin/sh

echo "=================================="
echo "Starting HTTP Server checking for 2-drive"
echo "=================================="

echo "=== DISK CHECK ==="
lsblk
echo "=================="
ip addr add 172.16.0.2/24 dev eth0
ip link set eth0 up
ip route add default via 172.16.0.1

echo 'nameserver 8.8.8.8' > /etc/resolv.conf

cd /root

python3 -m http.server 8080