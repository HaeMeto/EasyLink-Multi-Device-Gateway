//go:build windows

package services

import (
 "fmt"
 "os/exec"
 "strconv"
 "strings"
 "syscall"
 "unsafe"
)

var (
 kernel32 = syscall.NewLazyDLL("kernel32.dll")
 procOpenProcess = kernel32.NewProc("OpenProcess")
 procGetExitCodeProcess = kernel32.NewProc("GetExitCodeProcess")
 procCloseHandle = kernel32.NewProc("CloseHandle")
 procTerminateProcess = kernel32.NewProc("TerminateProcess")
)

const (
 processQueryLimitedInfo = 0x1000
 processTerminate = 0x0001
 stillActive = 259
)

func isProcessAlive(pid int) bool {
 if pid <= 0 {
 return false
 }
 handle, _, _ := procOpenProcess.Call(processQueryLimitedInfo, 0, uintptr(pid))
 if handle == 0 {
 return false
 }
 defer procCloseHandle.Call(handle)

 var exitCode uint32
 ret, _, _ := procGetExitCodeProcess.Call(handle, uintptr(unsafe.Pointer(&exitCode)))
 if ret == 0 {
 return false
 }
 return exitCode == stillActive
}

func terminateProcess(pid int) error {
 handle, _, _ := procOpenProcess.Call(processTerminate, 0, uintptr(pid))
 if handle == 0 {
 return fmt.Errorf("OpenProcess failed for pid %d", pid)
 }
 defer procCloseHandle.Call(handle)

 ret, _, _ := procTerminateProcess.Call(handle, 1)
 if ret == 0 {
 return fmt.Errorf("TerminateProcess failed for pid %d", pid)
 }
 return nil
}

func findPIDByPort(port int) (int, error) {
 cmd := exec.Command("netstat", "-ano")
 output, err := cmd.Output()
 if err != nil {
 return 0, fmt.Errorf("netstat failed: %w", err)
 }

 portStr := fmt.Sprintf(":%d", port)
 lines := strings.Split(string(output), "\n")

 for _, line := range lines {
 if !strings.Contains(line, "LISTENING") {
 continue
 }
 fields := strings.Fields(line)
 if len(fields) < 5 {
 continue
 }
 if !strings.HasSuffix(fields[1], portStr) {
 continue
 }
 pid, err := strconv.Atoi(fields[len(fields)-1])
 if err != nil {
 continue
 }
 return pid, nil
 }
 return 0, nil
}
