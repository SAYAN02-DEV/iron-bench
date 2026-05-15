#!/bin/bash

# =========================================================
# PRE-WARMED TAP DEVICE POOL
# =========================================================

set -e

# =========================================================
# CONFIG
# =========================================================

BRIDGE_DEV="br0"

HOST_IP="172.16.0.1/24"

POOL_SIZE=1000

# =========================================================
# CLEANUP OLD BRIDGE
# =========================================================

echo "Cleaning old bridge..."

sudo ip link del "$BRIDGE_DEV" 2>/dev/null || true

# =========================================================
# CREATE BRIDGE
# =========================================================

echo "Creating bridge..."

sudo ip link add name "$BRIDGE_DEV" type bridge

# =========================================================
# ASSIGN HOST IP
# =========================================================

echo "Assigning IP to bridge..."

sudo ip addr add "$HOST_IP" dev "$BRIDGE_DEV"

# =========================================================
# BRING BRIDGE UP
# =========================================================

echo "Bringing bridge up..."

sudo ip link set "$BRIDGE_DEV" up

# =========================================================
# CREATE TAP DEVICE POOL
# =========================================================

echo
echo "Creating $POOL_SIZE TAP devices..."
echo

for ((i=0; i<POOL_SIZE; i++))
do
    TAP_DEV="tap$i"

    echo "[$i/$POOL_SIZE] Creating $TAP_DEV"

    # -----------------------------------------
    # REMOVE OLD TAP IF EXISTS
    # -----------------------------------------

    sudo ip link del "$TAP_DEV" 2>/dev/null || true

    # -----------------------------------------
    # CREATE TAP
    # -----------------------------------------

    sudo ip tuntap add dev "$TAP_DEV" mode tap

    # -----------------------------------------
    # ATTACH TO BRIDGE
    # -----------------------------------------

    sudo ip link set "$TAP_DEV" master "$BRIDGE_DEV"

    # -----------------------------------------
    # BRING TAP UP
    # -----------------------------------------

    sudo ip link set "$TAP_DEV" up
done

# =========================================================
# ENABLE IP FORWARDING
# =========================================================

echo
echo "Enabling IP forwarding..."

sudo sysctl -w net.ipv4.ip_forward=1

# =========================================================
# ALLOW FORWARDING
# =========================================================

echo "Allowing packet forwarding..."

sudo iptables -P FORWARD ACCEPT

# =========================================================
# DETECT INTERNET INTERFACE
# =========================================================

HOST_IFACE=$(ip -j route list default | jq -r '.[0].dev')

echo "Detected outbound interface: $HOST_IFACE"

# =========================================================
# CONFIGURE NAT
# =========================================================

echo "Configuring NAT..."

sudo iptables -t nat -D POSTROUTING \
    -o "$HOST_IFACE" \
    -j MASQUERADE 2>/dev/null || true

sudo iptables -t nat -A POSTROUTING \
    -o "$HOST_IFACE" \
    -j MASQUERADE

# =========================================================
# DONE
# =========================================================

echo
echo "========================================="
echo "TAP Pool Ready"
echo "========================================="
echo

echo "Bridge : $BRIDGE_DEV"
echo "Host IP: 172.16.0.1"
echo "Pool   : $POOL_SIZE TAP devices"
echo

echo "Created:"
echo "tap0 -> tap$((POOL_SIZE-1))"

echo
echo "========================================="
echo "Example Usage"
echo "========================================="
echo

echo "VM #1  -> tap0"
echo "VM #2  -> tap1"
echo "VM #3  -> tap2"
echo "..."
echo

echo "Firecracker network config:"
echo
echo '{'
echo '  "iface_id":"net1",'
echo '  "guest_mac":"06:00:AC:10:00:02",'
echo '  "host_dev_name":"tap42"'
echo '}'

echo
echo "========================================="
echo "Pool Initialization Complete"
echo "========================================="
echo