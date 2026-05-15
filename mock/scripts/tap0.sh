#!/bin/bash



set -e


TAP_DEV="tap0"
BRIDGE_DEV="br0"

HOST_IP="172.16.0.1/24"



echo "Cleaning old interfaces..."

sudo ip link del "$TAP_DEV" 2>/dev/null || true
sudo ip link del "$BRIDGE_DEV" 2>/dev/null || true



echo "Creating TAP device..."

sudo ip tuntap add dev "$TAP_DEV" mode tap



echo "Creating bridge..."

sudo ip link add name "$BRIDGE_DEV" type bridge


echo "Attaching TAP to bridge..."

sudo ip link set "$TAP_DEV" master "$BRIDGE_DEV"



echo "Bringing interfaces up..."

sudo ip link set "$TAP_DEV" up
sudo ip link set "$BRIDGE_DEV" up


echo "Assigning IP to bridge..."

sudo ip addr add "$HOST_IP" dev "$BRIDGE_DEV"



echo "Enabling IP forwarding..."

sudo sysctl -w net.ipv4.ip_forward=1


echo "Allowing packet forwarding..."

sudo iptables -P FORWARD ACCEPT



HOST_IFACE=$(ip -j route list default | jq -r '.[0].dev')

echo "Detected outbound interface: $HOST_IFACE"


echo "Configuring NAT..."

sudo iptables -t nat -D POSTROUTING \
    -o "$HOST_IFACE" \
    -j MASQUERADE 2>/dev/null || true

sudo iptables -t nat -A POSTROUTING \
    -o "$HOST_IFACE" \
    -j MASQUERADE


echo
echo "========================================="
echo "Firecracker Networking Ready"
echo "========================================="
echo

ip addr show "$BRIDGE_DEV"

echo
echo "========================================="
echo "Topology"
echo "========================================="
echo
echo "Host Bridge : br0 -> 172.16.0.1"
echo "MicroVM IP  : 172.16.0.2"
echo
echo "Bridge : br0"
echo "TAP    : tap0"
echo

echo "========================================="
echo "Guest VM Network Commands"
echo "========================================="
echo

echo "Run INSIDE microVM:"
echo
echo "ip addr add 172.16.0.2/24 dev eth0"
echo "ip link set eth0 up"
echo "ip route add default via 172.16.0.1"
echo "echo 'nameserver 8.8.8.8' > /etc/resolv.conf"
echo

echo "========================================="
echo "Network Setup Complete"
echo "========================================="
echo