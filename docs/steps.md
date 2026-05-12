## first I will try to programmatically control a microVM

1. Wrote a basic go code
2. try to manually boot a microVM
3. copied the firecracker repo
4. built it
# Firecracker Manual MicroVM Boot Guide

## 1. Prerequisites

Check if KVM is enabled:

```bash
lsmod | grep kvm
```

Check `/dev/kvm` permissions:

```bash
ls -l /dev/kvm
```

Add current user to `kvm` group if needed:

```bash
sudo usermod -aG kvm $USER
newgrp kvm
```

Verify access:

```bash
[ -r /dev/kvm ] && [ -w /dev/kvm ] && echo "OK" || echo "FAIL"
```

---

## 2. Docker Permissions Fix

If Docker gives permission denied:

```bash
sudo usermod -aG docker $USER
newgrp docker
```

Verify:

```bash
docker ps
```

---

## 3. Build Firecracker

Clone repository:

```bash
git clone git@github.com:firecracker-microvm/firecracker.git
cd firecracker
```

Build Firecracker:

```bash
tools/devtool build --release
```

Find binary:

```bash
find build -name firecracker
```

Copy binary locally:

```bash
cp build/cargo_target/*/release/firecracker .
```

Verify:

```bash
./firecracker --version
```

---

# 4. Download Kernel + RootFS

Run entire block:

```bash
ARCH="$(uname -m)"
release_url="https://github.com/firecracker-microvm/firecracker/releases"

latest_version=$(basename $(curl -fsSLI -o /dev/null -w %{url_effective} ${release_url}/latest))
CI_VERSION=${latest_version%.*}

latest_kernel_key=$(curl "http://spec.ccfc.min.s3.amazonaws.com/?prefix=firecracker-ci/$CI_VERSION/$ARCH/vmlinux-&list-type=2" \
    | grep -oP "(?<=<Key>)(firecracker-ci/$CI_VERSION/$ARCH/vmlinux-[0-9]+\.[0-9]+\.[0-9]{1,3})(?=</Key>)" \
    | sort -V | tail -1)

# Download Linux kernel
wget "https://s3.amazonaws.com/spec.ccfc.min/${latest_kernel_key}"

latest_ubuntu_key=$(curl "http://spec.ccfc.min.s3.amazonaws.com/?prefix=firecracker-ci/$CI_VERSION/$ARCH/ubuntu-&list-type=2" \
    | grep -oP "(?<=<Key>)(firecracker-ci/$CI_VERSION/$ARCH/ubuntu-[0-9]+\.[0-9]+\.squashfs)(?=</Key>)" \
    | sort -V | tail -1)

ubuntu_version=$(basename $latest_ubuntu_key .squashfs | grep -oE '[0-9]+\.[0-9]+')

# Download Ubuntu rootfs
wget -O ubuntu-$ubuntu_version.squashfs.upstream \
"https://s3.amazonaws.com/spec.ccfc.min/$latest_ubuntu_key"

# Extract rootfs
unsquashfs ubuntu-$ubuntu_version.squashfs.upstream

# Generate SSH key
ssh-keygen -f id_rsa -N ""

# Inject public key into guest
cp -v id_rsa.pub squashfs-root/root/.ssh/authorized_keys

# Rename private key
mv -v id_rsa ./ubuntu-$ubuntu_version.id_rsa

# Create ext4 root filesystem
sudo chown -R root:root squashfs-root

truncate -s 1G ubuntu-$ubuntu_version.ext4

sudo mkfs.ext4 -d squashfs-root -F ubuntu-$ubuntu_version.ext4
```

Verify files:

```bash
ls
```

You should see:

```text
vmlinux-*
ubuntu-*.ext4
ubuntu-*.id_rsa
```

---

# 5. Start Firecracker

## IMPORTANT FIX

Do NOT use:

```text
pci=off
```

if using `--enable-pci`.

That caused the VM to immediately shut down.

---

## Terminal 1

```bash
API_SOCKET="/tmp/firecracker.socket"

rm -f $API_SOCKET

./firecracker --api-sock "${API_SOCKET}"
```

Keep terminal open.

---

# 6. Setup TAP Networking

## Terminal 2

Delete old TAP device if it already exists:

```bash
sudo ip link del tap0 2>/dev/null || true
```

Create TAP interface:

```bash
TAP_DEV="tap0"
TAP_IP="172.16.0.1"
MASK_SHORT="/30"

sudo ip tuntap add dev "$TAP_DEV" mode tap
sudo ip addr add "${TAP_IP}${MASK_SHORT}" dev "$TAP_DEV"
sudo ip link set dev "$TAP_DEV" up
```

Verify:

```bash
ip addr show tap0
```

Enable forwarding:

```bash
sudo sh -c "echo 1 > /proc/sys/net/ipv4/ip_forward"
sudo iptables -P FORWARD ACCEPT
```

Setup NAT:

```bash
HOST_IFACE=$(ip -j route list default | jq -r '.[0].dev')

sudo iptables -t nat -A POSTROUTING -o "$HOST_IFACE" -j MASQUERADE
```

---

# 7. Configure Logger

```bash
curl --unix-socket /tmp/firecracker.socket \
    -i \
    -X PUT 'http://localhost/logger' \
    -H 'Content-Type: application/json' \
    -d '{
        "log_path":"./firecracker.log",
        "level":"Debug"
    }'
```

---

# 8. Configure Boot Source

```bash
KERNEL=$(ls vmlinux-* | tail -1)

curl --unix-socket /tmp/firecracker.socket \
    -i \
    -X PUT 'http://localhost/boot-source' \
    -H 'Accept: application/json' \
    -H 'Content-Type: application/json' \
    -d "{
        \"kernel_image_path\":\"./$KERNEL\",
        \"boot_args\":\"console=ttyS0 reboot=k panic=1\"
    }"
```

---

# 9. Attach RootFS

```bash
ROOTFS=$(ls *.ext4 | tail -1)

curl --unix-socket /tmp/firecracker.socket \
    -i \
    -X PUT 'http://localhost/drives/rootfs' \
    -H 'Content-Type: application/json' \
    -d "{
        \"drive_id\":\"rootfs\",
        \"path_on_host\":\"./$ROOTFS\",
        \"is_root_device\":true,
        \"is_read_only\":false
    }"
```

---

# 10. Configure Network Interface

```bash
curl --unix-socket /tmp/firecracker.socket \
    -i \
    -X PUT 'http://localhost/network-interfaces/net1' \
    -H 'Content-Type: application/json' \
    -d '{
        "iface_id":"net1",
        "guest_mac":"06:00:AC:10:00:02",
        "host_dev_name":"tap0"
    }'
```

---

# 11. Start MicroVM

```bash
curl --unix-socket /tmp/firecracker.socket \
    -i \
    -X PUT 'http://localhost/actions' \
    -H 'Content-Type: application/json' \
    -d '{
        "action_type":"InstanceStart"
    }'
```

VM should now boot successfully.

---

# 12. Configure Guest Networking

Inside the VM console:

```bash
ip addr add 172.16.0.2/30 dev eth0
ip link set eth0 up
ip route add default via 172.16.0.1
```

Setup DNS:

```bash
echo 'nameserver 8.8.8.8' > /etc/resolv.conf
```

Test:

```bash
ping 8.8.8.8
ping google.com
```

---

# 13. SSH Into MicroVM

From host machine:

```bash
ssh -i ubuntu-*.id_rsa root@172.16.0.2
```

---

# Common Errors & Fixes

## Docker Permission Denied

Fix:

```bash
sudo usermod -aG docker $USER
newgrp docker
```

---

## `/dev/kvm` Permission Denied

Fix:

```bash
sudo usermod -aG kvm $USER
newgrp kvm
```

---

## `tap0 Device or resource busy`

Delete old TAP device:

```bash
sudo ip link del tap0
```

---

## VM Immediately Exits

Cause:

```text
pci=off
```

while using:

```text
--enable-pci
```

Fix:
- remove `pci=off`
  OR
- don't use `--enable-pci`

---

## SSH: No route to host

Cause:
Guest networking not configured.

Fix inside VM:

```bash
ip addr add 172.16.0.2/30 dev eth0
ip link set eth0 up
ip route add default via 172.16.0.1
```

# Attaching payload with the ext so that it prints message at bootup

```bash
# =========================================================
# FIRECRACKER MICROVM HELLO WORLD PAYLOAD EXECUTION
# =========================================================

# ---------------------------------------------------------
# STEP 1 — Stop old microVM (if running)
# ---------------------------------------------------------

pkill firecracker 2>/dev/null || true

rm -f /tmp/firecracker.socket

sudo ip link del tap0 2>/dev/null || true


# =========================================================
# STEP 2 — Create Dummy Payload
# This script will run INSIDE the microVM automatically.
# =========================================================

cat > payload.sh << 'EOF'
#!/bin/sh

echo "=================================="
echo "Hello from Firecracker MicroVM"
echo "=================================="

sleep 2

poweroff -f
EOF

chmod +x payload.sh


# =========================================================
# STEP 3 — Mount the ext4 root filesystem
# We modify the VM filesystem from the host machine.
# =========================================================

mkdir -p rootfs_mount

sudo mount ubuntu-*.ext4 rootfs_mount


# =========================================================
# STEP 4 — Inject Payload into the microVM filesystem
# =========================================================

sudo cp payload.sh rootfs_mount/root/

sudo chmod +x rootfs_mount/root/payload.sh


# =========================================================
# STEP 5 — Configure Auto-start
# rc.local runs automatically during Linux boot.
# =========================================================

sudo tee rootfs_mount/etc/rc.local > /dev/null << 'EOF'
#!/bin/sh -e

/root/payload.sh

exit 0
EOF

sudo chmod +x rootfs_mount/etc/rc.local


# =========================================================
# STEP 6 — Unmount filesystem
# Changes are now permanently stored in ext4 image.
# =========================================================

sudo umount rootfs_mount


# =========================================================
# STEP 7 — Start Firecracker
# Terminal 1
# =========================================================

API_SOCKET="/tmp/firecracker.socket"

rm -f $API_SOCKET

./firecracker --api-sock "${API_SOCKET}"


# =========================================================
# STEP 8 — Setup TAP networking
# Terminal 2
# =========================================================

sudo ip link del tap0 2>/dev/null || true

TAP_DEV="tap0"
TAP_IP="172.16.0.1"
MASK_SHORT="/30"

sudo ip tuntap add dev "$TAP_DEV" mode tap

sudo ip addr add "${TAP_IP}${MASK_SHORT}" dev "$TAP_DEV"

sudo ip link set dev "$TAP_DEV" up


# =========================================================
# STEP 9 — Enable forwarding + NAT
# =========================================================

sudo sh -c "echo 1 > /proc/sys/net/ipv4/ip_forward"

sudo iptables -P FORWARD ACCEPT

HOST_IFACE=$(ip -j route list default | jq -r '.[0].dev')

sudo iptables -t nat -A POSTROUTING \
    -o "$HOST_IFACE" \
    -j MASQUERADE


# =========================================================
# STEP 10 — Configure Firecracker logger
# =========================================================

curl --unix-socket /tmp/firecracker.socket \
    -i \
    -X PUT 'http://localhost/logger' \
    -H 'Content-Type: application/json' \
    -d '{
        "log_path":"./firecracker.log",
        "level":"Debug"
    }'


# =========================================================
# STEP 11 — Configure Boot Source
# Attach Linux kernel to microVM
# =========================================================

KERNEL=$(ls vmlinux-* | tail -1)

curl --unix-socket /tmp/firecracker.socket \
    -i \
    -X PUT 'http://localhost/boot-source' \
    -H 'Content-Type: application/json' \
    -d "{
        \"kernel_image_path\":\"./$KERNEL\",
        \"boot_args\":\"console=ttyS0 reboot=k panic=1\"
    }"


# =========================================================
# STEP 12 — Attach Root Filesystem
# This is the VM disk image.
# =========================================================

ROOTFS=$(ls *.ext4 | tail -1)

curl --unix-socket /tmp/firecracker.socket \
    -i \
    -X PUT 'http://localhost/drives/rootfs' \
    -H 'Content-Type: application/json' \
    -d "{
        \"drive_id\":\"rootfs\",
        \"path_on_host\":\"./$ROOTFS\",
        \"is_root_device\":true,
        \"is_read_only\":false
    }"


# =========================================================
# STEP 13 — Attach Virtual Network Interface
# =========================================================

curl --unix-socket /tmp/firecracker.socket \
    -i \
    -X PUT 'http://localhost/network-interfaces/net1' \
    -H 'Content-Type: application/json' \
    -d '{
        "iface_id":"net1",
        "guest_mac":"06:00:AC:10:00:02",
        "host_dev_name":"tap0"
    }'


# =========================================================
# STEP 14 — Start the microVM
# =========================================================

curl --unix-socket /tmp/firecracker.socket \
    -i \
    -X PUT 'http://localhost/actions' \
    -H 'Content-Type: application/json' \
    -d '{
        "action_type":"InstanceStart"
    }'


# =========================================================
# EXPECTED OUTPUT INSIDE FIRECRACKER CONSOLE
# =========================================================

# ==================================
# Hello from Firecracker MicroVM
# ==================================
#
# reboot: System halted
#
# "System halted" means:
# - payload executed successfully
# - microVM finished execution
# - guest OS shut down correctly
#
# =========================================================
# END
# =========================================================

```


## basic orchestrator
# Firecracker Go Orchestrator Setup

## 1. Create a Clean Base ext4 Image

This creates a fresh reusable VM image.

```bash
sudo rm -rf squashfs-root

unsquashfs ubuntu-*.squashfs.upstream

sudo chown -R root:root squashfs-root

truncate -s 1G ubuntu-clean.ext4

sudo mkfs.ext4 -d squashfs-root -F ubuntu-clean.ext4
```

---

# 2. Create a Disposable VM Image

Never modify the clean base image directly.

Create a working VM copy:

```bash
cp ubuntu-clean.ext4 vm.ext4
```

---

# 3. Mount the VM Filesystem

```bash
mkdir -p rootfs_mount

sudo mount vm.ext4 rootfs_mount
```

---

# 4. Create the Payload

This script will execute automatically inside the microVM.

```bash
cat > payload.sh << 'EOF'
#!/bin/sh

echo "=================================="
echo "Hello from Go Orchestrator VM"
echo "=================================="

sleep 2

poweroff -f
EOF
```

---

# 5. Inject Payload into the microVM

Copy payload into guest filesystem:

```bash
sudo cp payload.sh rootfs_mount/root/
```

Make executable:

```bash
sudo chmod +x rootfs_mount/root/payload.sh
```

---

# 6. Configure Auto-run on Boot

Create `/etc/rc.local` inside the guest VM:

```bash
sudo tee rootfs_mount/etc/rc.local > /dev/null << 'EOF'
#!/bin/sh -e

/root/payload.sh

exit 0
EOF
```

Make executable:

```bash
sudo chmod +x rootfs_mount/etc/rc.local
```

---

# 7. Unmount the Filesystem

Save all changes permanently into `vm.ext4`.

```bash
sudo umount rootfs_mount
```

---

# 8. Update Go Orchestrator Config

Inside your Go file:

```go
rootfsPath = "./ubuntu-clean.ext4"
```

Change to:

```go
rootfsPath = "./vm.ext4"
```

---

# 9. Setup TAP Networking

Run once before starting orchestrator:

```bash
sudo ip link del tap0 2>/dev/null || true

sudo ip tuntap add dev tap0 mode tap

sudo ip addr add 172.16.0.1/30 dev tap0

sudo ip link set dev tap0 up
```

---

# 10. Run the Orchestrator

```bash
go run orchestrator.go
```

---

# Expected Output

Inside the Firecracker console you should see:

```text
==================================
Hello from Go Orchestrator VM
==================================

reboot: System halted
```

`System halted` means:
- payload executed successfully
- microVM completed execution
- guest OS shut down correctly



## full automatic from image to run

# Firecracker Automatic Go Orchestrator

This project automatically:

- creates disposable microVM images
- injects payload scripts into the VM
- configures Linux auto-start
- boots Firecracker microVMs
- executes payloads automatically
- streams VM console output
- destroys temporary VM images after execution

---

# Project Architecture

```text
Payload Script
      ↓
Go Orchestrator
      ↓
Create disposable ext4 VM image
      ↓
Inject payload into VM filesystem
      ↓
Configure rc.local auto-start
      ↓
Boot Firecracker microVM
      ↓
Execute payload
      ↓
Stream VM console logs
      ↓
Cleanup VM image
```

---

# Requirements

- Linux with KVM enabled
- Firecracker built successfully
- Docker installed
- Go installed
- `ubuntu-clean.ext4` base image created
- TAP networking configured

---

# Important Paths

Current setup uses:

```text
Firecracker Binary:
    /home/sayan/firecracker/firecracker

Kernel:
    /home/sayan/firecracker/vmlinux-6.1.155

Base RootFS:
    /home/sayan/firecracker/ubuntu-clean.ext4

Disposable VM:
    /home/sayan/firecracker/vm.ext4
```

---

# 1. Create Clean Base Image

Run once.

```bash
sudo rm -rf squashfs-root

unsquashfs ubuntu-*.squashfs.upstream

sudo chown -R root:root squashfs-root

truncate -s 1G ubuntu-clean.ext4

sudo mkfs.ext4 -d squashfs-root -F ubuntu-clean.ext4
```

This creates:

```text
ubuntu-clean.ext4
```

This image should NEVER be modified directly.

---

# 2. TAP Networking Setup

Run before starting orchestrator.

```bash
sudo ip link del tap0 2>/dev/null || true

sudo ip tuntap add dev tap0 mode tap

sudo ip addr add 172.16.0.1/30 dev tap0

sudo ip link set dev tap0 up
```

---

# 3. Create Payload

Example:

```bash
#!/bin/sh

echo "=================================="
echo "Hello from Automatic Orchestrator"
echo "=================================="

sleep 2

poweroff -f
```

Save as:

```text
test_payload.sh
```

Make executable:

```bash
chmod +x test_payload.sh
```

---

# 4. Run Orchestrator

Example:

```bash
go run full_auto_main.go /home/sayan/Documents/projects/iron-bench/scripts/test_payload.sh
```

The payload path is passed as command-line argument:

```text
os.Args[1]
```

---

# What Happens Automatically

The orchestrator performs:

1. Creates disposable VM image from clean base
2. Mounts ext4 filesystem
3. Copies payload into guest VM
4. Creates `/etc/rc.local`
5. Unmounts filesystem
6. Starts Firecracker
7. Configures kernel + rootfs
8. Boots microVM
9. Executes payload
10. Streams VM console output
11. Deletes disposable VM image

---

# Expected Output

```text
==================================
Hello from Automatic Orchestrator
==================================

reboot: System halted
```

`System halted` means:
- payload executed successfully
- microVM shut down correctly

---

# Firecracker API Flow

The orchestrator communicates with Firecracker using a Unix socket:

```text
/tmp/firecracker.socket
```

API sequence:

```text
/logger
    ↓
/boot-source
    ↓
/drives/rootfs
    ↓
/actions
```

---

# Important Files

## `ubuntu-clean.ext4`

Immutable clean base image.

---

## `vm.ext4`

Disposable VM image created automatically for each run.

Deleted after VM exits.

---

## `/etc/rc.local`

Created automatically inside guest VM.

Used to auto-run payload on boot.

---

## `payload.sh`

User-provided script executed inside microVM.

---

# Current Limitations

Currently orchestrator does NOT:
- auto-create TAP networking
- support concurrent VMs
- capture stdout separately
- limit CPU/memory
- support snapshots
- auto-timeout long-running VMs

---

# Future Improvements

Planned features:

- dynamic TAP networking
- stdout/stderr capture
- VM IDs
- concurrent microVMs
- REST API
- timeout enforcement
- resource limits
- snapshot restore
- jailed execution
- web-based code execution

---

# Security Note

Current implementation is for learning/development purposes only.

Production systems should additionally use:
- Firecracker jailer
- cgroups
- seccomp
- namespaces
- UID/GID isolation
- snapshot-based booting
- read-only base images

---

# Cleanup

If networking issues occur:

```bash
sudo ip link del tap0
```

If stale socket exists:

```bash
rm -f /tmp/firecracker.socket
```

Kill old Firecracker processes:

```bash
pkill firecracker
```


### creating tap0 and br0

# Firecracker Networking Setup & microVM Connectivity

This guide explains how to:

- create TAP networking for Firecracker
- create a Linux bridge
- configure NAT/internet access
- attach Firecracker microVM to `tap0`
- configure guest networking
- verify internet connectivity from the microVM

---

# Network Topology

```text
microVM eth0
      ↓
Firecracker virtio-net
      ↓
tap0
      ↓
br0
      ↓
Host Linux
      ↓
iptables NAT
      ↓
Internet
```

---

# 1. Create Networking Setup Script

Create file:

```bash
nano tap0.sh
```

Paste:

```bash
#!/bin/bash

# =========================================================
# Firecracker Networking Setup
# =========================================================

set -e

# =========================================================
# CONFIG
# =========================================================

TAP_DEV="tap0"
BRIDGE_DEV="br0"

HOST_IP="172.16.0.1/24"

# =========================================================
# CLEANUP OLD INTERFACES
# =========================================================

echo "Cleaning old interfaces..."

sudo ip link del "$TAP_DEV" 2>/dev/null || true
sudo ip link del "$BRIDGE_DEV" 2>/dev/null || true

# =========================================================
# CREATE TAP DEVICE
# =========================================================

echo "Creating TAP device..."

sudo ip tuntap add dev "$TAP_DEV" mode tap

# =========================================================
# CREATE BRIDGE
# =========================================================

echo "Creating bridge..."

sudo ip link add name "$BRIDGE_DEV" type bridge

# =========================================================
# ATTACH TAP TO BRIDGE
# =========================================================

echo "Attaching TAP to bridge..."

sudo ip link set "$TAP_DEV" master "$BRIDGE_DEV"

# =========================================================
# BRING INTERFACES UP
# =========================================================

echo "Bringing interfaces up..."

sudo ip link set "$TAP_DEV" up
sudo ip link set "$BRIDGE_DEV" up

# =========================================================
# ASSIGN IP TO BRIDGE
# =========================================================

echo "Assigning IP to bridge..."

sudo ip addr add "$HOST_IP" dev "$BRIDGE_DEV"

# =========================================================
# ENABLE IP FORWARDING
# =========================================================

echo "Enabling IP forwarding..."

sudo sysctl -w net.ipv4.ip_forward=1

# =========================================================
# ALLOW FORWARDING
# =========================================================

echo "Allowing packet forwarding..."

sudo iptables -P FORWARD ACCEPT

# =========================================================
# DETECT HOST INTERNET INTERFACE
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
# SHOW RESULT
# =========================================================

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
```

---

# 2. Make Script Executable

```bash
chmod +x tap0.sh
```

---

# 3. Run Networking Script

```bash
./tap0.sh
```

Expected result:

```text
tap0 created
br0 created
172.16.0.1 assigned
NAT enabled
```

---

# 4. Attach `tap0` Inside Go Orchestrator

Add this BEFORE:

```go
put("/actions", ...)
```

Add:

```go
put("/network-interfaces/net1", `
{
	"iface_id":"net1",
	"guest_mac":"06:00:AC:10:00:02",
	"host_dev_name":"tap0"
}
`)
```

---

# What This Does

This attaches the Firecracker microVM network interface to:

```text
tap0
```

Packet flow becomes:

```text
VM eth0
   ↓
virtio-net
   ↓
tap0
   ↓
br0
   ↓
Host Network
   ↓
Internet
```

---

# 5. Start Orchestrator

Example:

```bash
go run full_auto_main.go /home/sayan/Documents/projects/iron-bench/scripts/test_payload.sh
```

---

# 6. Boot Into microVM

After boot you should get:

```text
root@ubuntu-fc-uvm:~#
```

---

# 7. Configure Guest Networking

Run INSIDE the microVM:

```bash
ip addr add 172.16.0.2/24 dev eth0

ip link set eth0 up

ip route add default via 172.16.0.1

echo 'nameserver 8.8.8.8' > /etc/resolv.conf
```

---

# 8. Test Host ↔ VM Connectivity

Inside VM:

```bash
ping 172.16.0.1
```

Expected:

```text
64 bytes from 172.16.0.1
```

This verifies:
- bridge works
- tap works
- host ↔ VM communication works

---

# 9. Test Internet Access

Inside VM:

```bash
ping google.com
```

Expected:

```text
64 bytes from google.com
```

This verifies:
- NAT works
- routing works
- DNS works
- internet access works

---

# Final Result

Your Firecracker microVM now has:

- isolated virtual networking
- host communication
- internet access
- Linux bridge networking
- TAP device connectivity
- virtio-net support

---

# Final Working Architecture

```text
microVM eth0
      ↓
Firecracker virtio-net
      ↓
tap0
      ↓
br0
      ↓
Host Linux
      ↓
iptables NAT
      ↓
Internet
```


## server as payload

# Firecracker HTTP Server microVM

This milestone demonstrates:

- automated payload injection into a microVM
- automatic payload execution using `rc.local`
- Firecracker networking using TAP + bridge
- exposing an HTTP server from inside the microVM
- accessing the microVM service from the host machine

---

# Goal

Boot a Firecracker microVM that automatically:

- configures networking
- starts a Python HTTP server
- exposes port `8080`

Then access it from the host machine using:

```bash
curl http://172.16.0.2:8080
```

---

# Architecture

```text
Host Machine
    ↓
Linux Bridge (br0)
    ↓
tap0
    ↓
Firecracker virtio-net
    ↓
microVM eth0
    ↓
Python HTTP Server
```

---

# Payload Script

Create payload:

```bash
nano test_payload.sh
```

Paste:

```bash
#!/bin/sh

echo "=================================="
echo "Starting HTTP Server"
echo "=================================="

# ==================================
# Configure Guest Networking
# ==================================

ip addr add 172.16.0.2/24 dev eth0

ip link set eth0 up

ip route add default via 172.16.0.1

echo 'nameserver 8.8.8.8' > /etc/resolv.conf

# ==================================
# Create Demo Web Page
# ==================================

echo "Hello from Firecracker microVM" > /root/index.html

# ==================================
# Start HTTP Server
# ==================================

cd /root

python3 -m http.server 8080
```

---

# Make Payload Executable

```bash
chmod +x test_payload.sh
```

---

# What The Payload Does

## Configure Networking

Sets:

- IP address
- default route
- DNS server

inside the microVM.

---

## Create Web Page

Creates:

```text
/root/index.html
```

with:

```text
Hello from Firecracker microVM
```

---

## Start HTTP Server

Runs:

```bash
python3 -m http.server 8080
```

inside the microVM.

---

# rc.local Injection

The Go orchestrator automatically creates:

```bash
#!/bin/sh -e

/root/payload.sh

exit 0
```

inside:

```text
/etc/rc.local
```

This makes the payload execute automatically during boot.

---

# Important Filesystem Flow

Correct orchestration lifecycle:

```text
copy clean ext4
↓
mount ext4
↓
inject payload
↓
inject rc.local
↓
chmod files
↓
sync
↓
unmount
↓
boot Firecracker
```

---

# Important Bug Fixed

Earlier bug:

```text
mount
↓
unmount
↓
inject payload
```

This caused:
- payload not entering ext4
- rc.local not existing inside VM
- manual server startup required

Correct fix:
- inject FIRST
- unmount LAST

---

# Correct Unmount Section

After payload injection:

```go
fmt.Println("Syncing filesystem...")

run("sync")

time.Sleep(1 * time.Second)

fmt.Println("Unmounting VM filesystem...")

run("sudo", "umount", "-l", mountDir)
```

---

# Start Networking Setup

Before booting VMs:

```bash
./tap0.sh
```

This creates:
- tap0
- br0
- NAT
- IP forwarding

---

# Start Orchestrator

Run:

```bash
go run full_auto_main.go /home/sayan/Documents/projects/iron-bench/scripts/test_payload.sh
```

---

# Expected VM Output

During boot:

```text
Starting HTTP Server
```

Then:

```text
Serving HTTP on 0.0.0.0 port 8080
```

---

# Test From Host Machine

On HOST terminal:

```bash
curl http://172.16.0.2:8080
```

---

# Expected Result

```text
Hello from Firecracker microVM
```

---

# What This Proves

Successfully demonstrated:

- Firecracker networking
- TAP device connectivity
- Linux bridge networking
- virtio-net support
- guest networking automation
- automatic payload execution
- service exposure from microVM
- host → VM HTTP communication

---

# Final Working Network

```text
Host Machine
    ↓
br0
    ↓
tap0
    ↓
Firecracker virtio-net
    ↓
microVM eth0
    ↓
Python HTTP Server
```

---

# Milestone Achieved

You can now:

```text
curl the microVM from the host machine
and receive a 200 OK HTTP response
```

This is a major infrastructure milestone toward:
- sandbox runtimes
- serverless execution
- isolated compute infrastructure
- lightweight VM orchestration.

## 2-drive system payload injection
1. Mount the Base OS drive (Read-Write on your host):
   Bash
```
sudo mount /home/sayan/firecracker/ubuntu-clean.ext4 /home/sayan/firecracker/rootfs_mount
```
2. Create the empty directory permanently:
   Bash
```
sudo mkdir -p /home/sayan/firecracker/rootfs_mount/workspace
```

3. rc.local
```aiignore
sudo tee /home/sayan/firecracker/rootfs_mount/etc/rc.local > /dev/null << 'EOF'
#!/bin/sh -e

# Mount the second hard drive (the tiny payload drive) to our pre-existing folder
mount /dev/vdb /workspace

# Execute the contestant's payload
/workspace/payload.sh

exit 0
EOF
```

4. make it executable
```aiignore
sudo chmod +x rc.local
```
5. unmount
```aiignore
sudo umount /home/sayan/firecracker/rootfs_mount
```
new_main.go file