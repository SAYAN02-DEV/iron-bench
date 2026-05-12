package main

//
//import (
//	"bytes"
//	"context"
//	"fmt"
//	"io"
//	"net"
//	"net/http"
//	"os"
//	"os/exec"
//	"time"
//)
//
//const (
//	socketPath = "/tmp/firecracker.socket"
//	kernelPath = "/home/sayan/firecracker/vmlinux-6.1.155"
//
//	// USE YOUR CLEAN IMAGE
//	rootfsPath = "/home/sayan/firecracker/vm.ext4"
//
//	firecrackerBinary = "/home/sayan/firecracker/firecracker"
//)
//
//func put(endpoint string, body string) {
//	tr := &http.Transport{
//		DialContext: func(_ context.Context, _, _ string) (net.Conn, error) {
//			return net.Dial("unix", socketPath)
//		},
//	}
//
//	client := &http.Client{
//		Transport: tr,
//	}
//
//	req, err := http.NewRequest(
//		"PUT",
//		"http://localhost"+endpoint,
//		bytes.NewBuffer([]byte(body)),
//	)
//
//	if err != nil {
//		panic(err)
//	}
//
//	req.Header.Set("Content-Type", "application/json")
//
//	resp, err := client.Do(req)
//
//	if err != nil {
//		panic(err)
//	}
//
//	defer resp.Body.Close()
//
//	respBody, _ := io.ReadAll(resp.Body)
//
//	fmt.Println("===================================")
//	fmt.Println("API:", endpoint)
//	fmt.Println("STATUS:", resp.Status)
//	fmt.Println(string(respBody))
//	fmt.Println("===================================")
//}
//
//func main() {
//
//	os.Remove(socketPath)
//
//	fmt.Println("Starting Firecracker...")
//
//	cmd := exec.Command(
//		firecrackerBinary,
//		"--api-sock",
//		socketPath,
//	)
//
//	// CONNECT FIRECRACKER OUTPUT TO HOST TERMINAL
//	cmd.Stdout = os.Stdout
//	cmd.Stderr = os.Stderr
//
//	err := cmd.Start()
//
//	if err != nil {
//		panic(err)
//	}
//
//	fmt.Println("Firecracker PID:", cmd.Process.Pid)
//
//	// WAIT FOR SOCKET TO BE READY
//	time.Sleep(2 * time.Second)
//
//	put("/logger", `
//{
//	"log_path":"./firecracker.log",
//	"level":"Debug"
//}
//`)
//
//	put("/boot-source", fmt.Sprintf(`
//{
//	"kernel_image_path":"%s",
//	"boot_args":"console=ttyS0 reboot=k panic=1"
//}
//`, kernelPath))
//
//	put("/drives/rootfs", fmt.Sprintf(`
//{
//	"drive_id":"rootfs",
//	"path_on_host":"%s",
//	"is_root_device":true,
//	"is_read_only":false
//}
//`, rootfsPath))
//
//	put("/actions", `
//{
//	"action_type":"InstanceStart"
//}
//`)
//
//	fmt.Println()
//	fmt.Println("===================================")
//	fmt.Println("MicroVM Started")
//	fmt.Println("===================================")
//
//	// WAIT FOR VM TO EXIT
//	cmd.Wait()
//
//	fmt.Println()
//	fmt.Println("===================================")
//	fmt.Println("MicroVM Exited")
//	fmt.Println("===================================")
//}
