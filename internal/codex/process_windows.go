package codex

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// hideProcessWindow 隐藏子进程窗口并禁止创建控制台
func hideProcessWindow(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
	}
}
