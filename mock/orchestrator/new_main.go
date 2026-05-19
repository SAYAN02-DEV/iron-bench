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
	socketPath        = "/tmp/firecracker.socket"
	firecrackerBinary = "/home/sayan/firecracker/firecracker"
	kernelPath        = "/home/sayan/firecracker/vmlinux-6.1.155"
	baseRootfs        = "/home/sayan/firecracker/ubuntu-clean.ext4"
	payloadDrive      = "/home/sayan/firecracker/payload_drive.ext4" // THE NEW TINY DRIVE
	mountDir          = "/home/sayan/firecracker/rootfs_mount"
)

// Build contestant's Go code on the host.
func buildContestantBinary(srcFile string, outputPath string) error {
	cmd := exec.Command("go", "build", "-o", outputPath, srcFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// FIRECRACKER API HELPER

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

// RUN SHELL COMMAND

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

// Inject the built binary into the payload drive.
func injectBinary(binaryPath string, mountPath string) {
	guestBinaryPath := filepath.Join(mountPath, "orderbook-server")
	run("sudo", "cp", binaryPath, guestBinaryPath)
	run("sudo", "chmod", "+x", guestBinaryPath)
}

// MAIN

func main() {

	// PAYLOAD PATH
	defaultPayloadPath := ""
	defaultServerPath := ""
	if cwd, err := os.Getwd(); err == nil {
		repoRoot := cwd
		if filepath.Base(cwd) == "orchestrator" && filepath.Base(filepath.Dir(cwd)) == "mock" {
			repoRoot = filepath.Dir(filepath.Dir(cwd))
		}
		defaultPayloadPath = filepath.Join(repoRoot, "mock", "scripts", "test_payload.sh")
		defaultServerPath = filepath.Join(repoRoot, "mock", "main.go")
	}

	payloadPath := defaultPayloadPath
	serverMain := defaultServerPath
	if len(os.Args) >= 2 {
		payloadPath = os.Args[1]
	}
	if payloadPath == "" || serverMain == "" {
		fmt.Println("Usage:")
		fmt.Println("go run main.go <payload-file>")
		return
	}

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("PAYLOAD:", payloadPath)
	fmt.Println("===================================")
	fmt.Println()

	// CLEANUP OLD FILES
	os.Remove(socketPath)
	os.Remove(payloadDrive) // Clean up any old tiny drives

	// 1. CREATE PAYLOAD DRIVE (Room for binary + payload)
	fmt.Println("Creating 64MB payload workspace...")
	run("dd", "if=/dev/zero", "of="+payloadDrive, "bs=1M", "count=64")
	run("mkfs.ext4", payloadDrive)

	// CREATE MOUNT DIRECTORY
	os.MkdirAll(mountDir, 0755)

	// 2. MOUNT THE TINY DRIVE (Not the big OS drive!)
	fmt.Println("Mounting tiny payload drive...")
	run("sudo", "mount", payloadDrive, mountDir)

	// Build contestant binary and inject it into the payload drive.
	compiledBinary := filepath.Join(os.TempDir(), "orderbook-server")
	if err := buildContestantBinary(serverMain, compiledBinary); err != nil {
		panic(err)
	}
	injectBinary(compiledBinary, mountDir)

	// READ PAYLOAD FILE
	fmt.Println("Reading payload...")
	payloadData, err := os.ReadFile(payloadPath)
	if err != nil {
		panic(err)
	}

	// 3. INJECT PAYLOAD INTO THE TINY DRIVE
	fmt.Println("Injecting payload into workspace...")
	guestPayloadPath := filepath.Join(mountDir, "payload.sh")

	// We cannot use os.WriteFile directly into mountDir because it is owned by root.
	// We write to a temporary file first, then use sudo cp to inject it.
	tempPayload := "/tmp/temp_payload.sh"
	err = os.WriteFile(tempPayload, payloadData, 0755)
	if err != nil {
		panic(err)
	}

	// Copy into the root-owned mount directory
	run("sudo", "cp", tempPayload, guestPayloadPath)

	// Ensure the payload is executable
	run("sudo", "chmod", "+x", guestPayloadPath)

	// 4. UNMOUNT THE TINY DRIVE
	fmt.Println("Unmounting payload drive...")
	run("sudo", "umount", mountDir)

	// START FIRECRACKER
	fmt.Println("Starting Firecracker...")
	cmd := exec.Command(
		firecrackerBinary,
		"--api-sock",
		socketPath,
	)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err = cmd.Start()
	if err != nil {
		panic(err)
	}

	fmt.Println("Firecracker PID:", cmd.Process.Pid)

	time.Sleep(2 * time.Second)

	// LOGGER
	put("/logger", `
{
  "log_path":"./firecracker.log",
  "level":"Debug"
}
`)

	// BOOT SOURCE
	put("/boot-source", fmt.Sprintf(`
{
  "kernel_image_path":"%s",
  "boot_args":"console=ttyS0 reboot=k panic=1"
}
`, kernelPath))

	// DRIVE 1: BASE OS (Strictly Read-Only)
	put("/drives/rootfs", fmt.Sprintf(`
{
  "drive_id":"rootfs",
  "path_on_host":"%s",
  "is_root_device":true,
  "is_read_only":true
}
`, baseRootfs))

	// DRIVE 2: THE PAYLOAD (Read-Write)
	put("/drives/payload", fmt.Sprintf(`
{
  "drive_id":"payload",
  "path_on_host":"%s",
  "is_root_device":false,
  "is_read_only":false
}
`, payloadDrive))

	// NETWORK INTERFACE
	put("/network-interfaces/net1", `
{
  "iface_id":"net1",
  "guest_mac":"06:00:AC:10:00:02",
  "host_dev_name":"tap0"
}
`)

	// START MICROVM
	put("/actions", `
{
  "action_type":"InstanceStart"
}
`)

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("MicroVM Started (Two-Drive Method)")
	fmt.Println("===================================")

	cmd.Wait()

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("MicroVM Exited")
	fmt.Println("===================================")

	// CLEANUP ONLY THE TINY DRIVE
	fmt.Println("Cleaning up payload workspace...")
	os.Remove(payloadDrive)
}
