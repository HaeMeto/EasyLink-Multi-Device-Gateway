//go:build windows

package services

import "syscall"

var sysProcAttr = &syscall.SysProcAttr{
 HideWindow: true,
}
