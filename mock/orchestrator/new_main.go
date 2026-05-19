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
	"sync"
	"time"
)

const (
	firecrackerBinary = "/home/sayan/firecracker/firecracker"
	kernelPath        = "/home/sayan/firecracker/vmlinux-6.1.155"
	baseRootfs        = "/home/sayan/firecracker/ubuntu-clean.ext4"
)

// Build contestant's Go code on the host.
func buildContestantBinary(srcFile string, outputPath string) error {
	cmd := exec.Command("go", "build", "-o", outputPath, srcFile)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

type vmConfig struct {
	id           string
	serverMain   string
	payloadPath  string
	socketPath   string
	payloadDrive string
	mountDir     string
	tapDevice    string
	guestIP      string
	guestMAC     string
	logPath      string
}

// FIRECRACKER API HELPER

func put(socketPath string, endpoint string, body string) {
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

func payloadScript(guestIP string) []byte {
	return []byte(fmt.Sprintf(`#!/bin/sh
mkdir -p /workspace
mount /dev/vdb /workspace

# Configure networking
ip addr add %s/24 dev eth0
ip link set eth0 up
ip route add default via 172.16.0.1
echo 'nameserver 8.8.8.8' > /etc/resolv.conf

# Run the contestant's binary directly
/workspace/orderbook-server
`, guestIP))
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

func injectPayload(payloadPath string, mountPath string) {
	guestPayloadPath := filepath.Join(mountPath, "payload.sh")
	run("sudo", "cp", payloadPath, guestPayloadPath)
	run("sudo", "chmod", "+x", guestPayloadPath)
}

func waitAllHealthy(configs []vmConfig) {
	var wg sync.WaitGroup
	client := &http.Client{Timeout: 2 * time.Second}

	for _, cfg := range configs {
		cfg := cfg
		wg.Add(1)
		go func() {
			defer wg.Done()
			healthURL := fmt.Sprintf("http://%s:8080/health", cfg.guestIP)
			for {
				resp, err := client.Get(healthURL)
				if err == nil {
					if resp.StatusCode == http.StatusOK {
						resp.Body.Close()
						return
					}
					resp.Body.Close()
				}
				time.Sleep(500 * time.Millisecond)
			}
		}()
	}

	wg.Wait()
}

type processInfo struct {
	cmd     *exec.Cmd
	logFile *os.File
}

func startFirecracker(cfg vmConfig) (processInfo, error) {
	cmd := exec.Command(
		firecrackerBinary,
		"--api-sock",
		cfg.socketPath,
	)

	logFile, err := os.OpenFile(
		fmt.Sprintf("./vm_%s.log", cfg.id),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC,
		0644,
	)
	if err != nil {
		return processInfo{}, err
	}

	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return processInfo{}, err
	}

	return processInfo{cmd: cmd, logFile: logFile}, nil
}

// MAIN

func main() {

	// PAYLOAD PATHS + SERVER PATHS
	defaultServerPath := ""
	if cwd, err := os.Getwd(); err == nil {
		repoRoot := cwd
		if filepath.Base(cwd) == "orchestrator" && filepath.Base(filepath.Dir(cwd)) == "mock" {
			repoRoot = filepath.Dir(filepath.Dir(cwd))
		}
		defaultServerPath = filepath.Join(repoRoot, "mock", "users")
	}

	if defaultServerPath == "" {
		fmt.Println("Usage:")
		fmt.Println("go run main.go")
		return
	}

	userIDs := []string{"user1", "user2", "user3", "user4", "user5"}
	configs := make([]vmConfig, 0, len(userIDs))
	for i, id := range userIDs {
		guestIP := fmt.Sprintf("172.16.0.%d", 2+i)
		payloadPath := filepath.Join(os.TempDir(), fmt.Sprintf("payload_%s.sh", id))
		if err := os.WriteFile(payloadPath, payloadScript(guestIP), 0755); err != nil {
			panic(err)
		}

		configs = append(configs, vmConfig{
			id:           id,
			serverMain:   filepath.Join(defaultServerPath, id+".go"),
			payloadPath:  payloadPath,
			socketPath:   filepath.Join("/tmp", "firecracker_"+id+".socket"),
			payloadDrive: filepath.Join("/home/sayan/firecracker", "payload_drive_"+id+".ext4"),
			mountDir:     filepath.Join("/home/sayan/firecracker", "rootfs_mount_"+id),
			tapDevice:    fmt.Sprintf("tap%d", i),
			guestIP:      guestIP,
			guestMAC:     fmt.Sprintf("06:00:AC:10:00:%02x", 2+i),
			logPath:      fmt.Sprintf("./firecracker_%s.log", id),
		})
	}

	for _, cfg := range configs {
		cfg := cfg
		defer os.Remove(cfg.payloadDrive)
	}

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("Phase 1: Prepare payload drives")
	fmt.Println("===================================")
	fmt.Println()

	for _, cfg := range configs {
		fmt.Println("Preparing:", cfg.id)

		// CLEANUP OLD FILES
		os.Remove(cfg.socketPath)
		os.Remove(cfg.payloadDrive)

		// 1. CREATE PAYLOAD DRIVE (Room for binary + payload)
		fmt.Println("Creating 64MB payload workspace for", cfg.id, "...")
		run("dd", "if=/dev/zero", "of="+cfg.payloadDrive, "bs=1M", "count=64")
		run("mkfs.ext4", cfg.payloadDrive)

		// CREATE MOUNT DIRECTORY
		os.MkdirAll(cfg.mountDir, 0755)

		// 2. MOUNT THE PAYLOAD DRIVE
		fmt.Println("Mounting payload drive for", cfg.id, "...")
		run("sudo", "mount", cfg.payloadDrive, cfg.mountDir)
		defer func(mountDir string) {
			_ = exec.Command("sudo", "umount", mountDir).Run()
		}(cfg.mountDir)

		// Build contestant binary and inject it into the payload drive.
		compiledBinary := filepath.Join(os.TempDir(), "orderbook-server-"+cfg.id)
		if err := buildContestantBinary(cfg.serverMain, compiledBinary); err != nil {
			panic(err)
		}
		injectBinary(compiledBinary, cfg.mountDir)
		injectPayload(cfg.payloadPath, cfg.mountDir)

		// 4. UNMOUNT THE PAYLOAD DRIVE
		fmt.Println("Unmounting payload drive for", cfg.id, "...")
		run("sudo", "umount", cfg.mountDir)
	}

	var (
		mu        sync.Mutex
		processes []processInfo
		wg        sync.WaitGroup
	)
	bootErrors := make(chan error, len(configs))

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("Phase 2: Boot all microVMs")
	fmt.Println("===================================")
	fmt.Println()

	for _, cfg := range configs {
		cfg := cfg
		wg.Add(1)
		go func() {
			defer wg.Done()

			fmt.Println("Starting Firecracker for", cfg.id, "...")
			proc, err := startFirecracker(cfg)
			if err != nil {
				bootErrors <- err
				return
			}

			mu.Lock()
			processes = append(processes, proc)
			mu.Unlock()

			fmt.Println("Firecracker PID:", proc.cmd.Process.Pid)

			time.Sleep(2 * time.Second)

			// LOGGER
			put(cfg.socketPath, "/logger", fmt.Sprintf(`
{
  "log_path":"%s",
  "level":"Debug"
}
`, cfg.logPath))

			// BOOT SOURCE
			put(cfg.socketPath, "/boot-source", fmt.Sprintf(`
{
  "kernel_image_path":"%s",
  "boot_args":"console=ttyS0 reboot=k panic=1"
}
`, kernelPath))

			// DRIVE 1: BASE OS (Strictly Read-Only)
			put(cfg.socketPath, "/drives/rootfs", fmt.Sprintf(`
{
  "drive_id":"rootfs",
  "path_on_host":"%s",
  "is_root_device":true,
  "is_read_only":true
}
`, baseRootfs))

			// DRIVE 2: THE PAYLOAD (Read-Write)
			put(cfg.socketPath, "/drives/payload", fmt.Sprintf(`
{
  "drive_id":"payload",
  "path_on_host":"%s",
  "is_root_device":false,
  "is_read_only":false
}
`, cfg.payloadDrive))

			// NETWORK INTERFACE
			put(cfg.socketPath, "/network-interfaces/net1", fmt.Sprintf(`
{
  "iface_id":"net1",
  "guest_mac":"%s",
  "host_dev_name":"%s"
}
`, cfg.guestMAC, cfg.tapDevice))

			// START MICROVM
			put(cfg.socketPath, "/actions", `
{
  "action_type":"InstanceStart"
}
`)

			fmt.Println()
			fmt.Println("===================================")
			fmt.Println("MicroVM Started:", cfg.id)
			fmt.Println("IP:", cfg.guestIP)
			fmt.Println("TAP:", cfg.tapDevice)
			fmt.Println("===================================")
		}()
	}

	wg.Wait()
	close(bootErrors)
	for err := range bootErrors {
		if err != nil {
			panic(err)
		}
	}

	fmt.Println()
	fmt.Println("===================================")
	fmt.Println("Phase 3: Wait for health checks")
	fmt.Println("===================================")
	fmt.Println()
	waitAllHealthy(configs)

	fmt.Println("All VMs healthy — bot fleet would fire here")

	for _, proc := range processes {
		proc.cmd.Wait()
		proc.logFile.Close()
	}

	for _, cfg := range configs {
		fmt.Println()
		fmt.Println("===================================")
		fmt.Println("MicroVM Exited:", cfg.id)
		fmt.Println("===================================")

		// CLEANUP ONLY THE TINY DRIVE
		fmt.Println("Cleaning up payload workspace for", cfg.id, "...")
		os.Remove(cfg.payloadDrive)
	}
}
