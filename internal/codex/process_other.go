//go:build !windows

package codex

import (
	"os/exec"
)

// hideProcessWindow 在非 Windows 平台保留默认启动行为
func hideProcessWindow(cmd *exec.Cmd) {}
