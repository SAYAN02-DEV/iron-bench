package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

const (
	socketPath = "/tmp/firecracker.socket"

	// FIRECRACKER BINARY
	firecrackerBinary = "/home/sayan/firecracker/firecracker"

	// LINUX KERNEL
	kernelPath = "/home/sayan/firecracker/vmlinux-6.1.155"

	// CLEAN BASE IMAGE
	baseRootfs = "/home/sayan/firecracker/ubuntu-clean.ext4"

	// DISPOSABLE VM IMAGE
	vmRootfs = "/home/sayan/firecracker/vm.ext4"

	// MOUNT DIRECTORY
	mountDir = "/home/sayan/firecracker/rootfs_mount"
)

// =========================================================
// FIRECRACKER API HELPER
// =========================================================

func put(endpoint string, body string) {
	tr := &http.Transport{
		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
			return net.Dial("unix", socketPath)
		},
	}

	client := &http.Client{
		Transport: tr,
	}

	req, err := http.NewRequest(
		"PUT",
		"http://localhost"+endpoint,
		bytes.NewBuffer([]byte(body)),
	)

	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)

	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	fmt.Println("===================================")
	fmt.Println("API:", endpoint)
	fmt.Println("STATUS:", resp.Status)
	fmt.Println(string(respBody))
	fmt.Println("===================================")
}

// =========================================================
// RUN SHELL COMMAND
// =========================================================

func run(name string, args ...string) {
	fmt.Println("RUN:", name, args)

	cmd := exec.Command(name, args...)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()

	if err != nil {
		panic(err)
	}
}

// =========================================================
// COPY FILE
// =========================================================

func copyFile(src string, dst string) {
	input, err := os.ReadFile(src)

	if err != nil {
		panic(err)
	}

	err = os.WriteFile(dst, input, 0644)

	if err != nil {
		panic(err)
	}
}

// =========================================================
// MAIN
// =========================================================

func main() {

	// =====================================================
	// PAYLOAD PATH
	// =====================================================

	if len(os.Args) < 2 {
		fmt.Println("Usage:")
		fmt.Println("go run main.go <payload-file>")
		return
	}

	payloadPath := os.Args[1]

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("PAYLOAD:", payloadPath)
	fmt.Println("===================================")
	fmt.Println()

	// =====================================================
	// CLEANUP OLD FILES
	// =====================================================

	os.Remove(socketPath)
	os.Remove(vmRootfs)

	// =====================================================
	// CREATE DISPOSABLE VM IMAGE
	// =====================================================

	fmt.Println("Creating disposable VM image...")

	copyFile(baseRootfs, vmRootfs)

	// =====================================================
	// CREATE MOUNT DIRECTORY
	// =====================================================

	os.MkdirAll(mountDir, 0755)

	// =====================================================
	// MOUNT VM IMAGE
	// =====================================================

	fmt.Println("Mounting VM filesystem...")

	run("sudo", "mount", vmRootfs, mountDir)

	// =====================================================
	// READ PAYLOAD FILE
	// =====================================================

	fmt.Println("Reading payload...")

	payloadData, err := os.ReadFile(payloadPath)

	if err != nil {
		panic(err)
	}

	// =====================================================
	// CREATE TEMP PAYLOAD FILE
	// =====================================================

	tempPayload := "/tmp/payload.sh"

	err = os.WriteFile(tempPayload, payloadData, 0755)

	if err != nil {
		panic(err)
	}

	// =====================================================
	// COPY PAYLOAD INTO VM
	// =====================================================

	fmt.Println("Injecting payload into VM...")

	guestPayloadPath := filepath.Join(
		mountDir,
		"root",
		"payload.sh",
	)

	run("sudo", "cp", tempPayload, guestPayloadPath)

	run("sudo", "chmod", "+x", guestPayloadPath)

	// =====================================================
	// CREATE rc.local
	// =====================================================

	fmt.Println("Creating rc.local...")

	rcLocal := `#!/bin/sh -e

/root/payload.sh

exit 0
`

	tempRcLocal := "/tmp/rc.local"

	err = os.WriteFile(
		tempRcLocal,
		[]byte(rcLocal),
		0755,
	)

	if err != nil {
		panic(err)
	}

	rcLocalPath := filepath.Join(
		mountDir,
		"etc",
		"rc.local",
	)

	run("sudo", "cp", tempRcLocal, rcLocalPath)

	run("sudo", "chmod", "+x", rcLocalPath)

	// =====================================================
	// UNMOUNT FILESYSTEM
	// =====================================================

	fmt.Println("Unmounting VM filesystem...")

	run("sudo", "umount", mountDir)

	// =====================================================
	// START FIRECRACKER
	// =====================================================

	fmt.Println("Starting Firecracker...")

	cmd := exec.Command(
		firecrackerBinary,
		"--api-sock",
		socketPath,
	)

	// STREAM VM OUTPUT TO TERMINAL
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Start()

	if err != nil {
		panic(err)
	}

	fmt.Println("Firecracker PID:", cmd.Process.Pid)

	// WAIT FOR SOCKET
	time.Sleep(2 * time.Second)

	// =====================================================
	// LOGGER
	// =====================================================

	put("/logger", `
{
	"log_path":"./firecracker.log",
	"level":"Debug"
}
`)

	// =====================================================
	// BOOT SOURCE
	// =====================================================

	put("/boot-source", fmt.Sprintf(`
{
	"kernel_image_path":"%s",
	"boot_args":"console=ttyS0 reboot=k panic=1"
}
`, kernelPath))

	// =====================================================
	// ROOTFS
	// =====================================================

	put("/drives/rootfs", fmt.Sprintf(`
{
	"drive_id":"rootfs",
	"path_on_host":"%s",
	"is_root_device":true,
	"is_read_only":false
}
`, vmRootfs))

	// =====================================================
	// START MICROVM
	// =====================================================

	put("/actions", `
{
	"action_type":"InstanceStart"
}
`)

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("MicroVM Started")
	fmt.Println("===================================")

	// =====================================================
	// WAIT FOR VM TO EXIT
	// =====================================================

	cmd.Wait()

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("MicroVM Exited")
	fmt.Println("===================================")

	// =====================================================
	// CLEANUP
	// =====================================================

	fmt.Println("Cleaning up VM image...")

	os.Remove(vmRootfs)
}
