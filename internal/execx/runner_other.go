//go:build !windows

package execx

import "os/exec"

// setupPlatformProcess 非 Windows 环境下的基础进程清理
func setupPlatformProcess(cmd *exec.Cmd) (func(), error) {
	cleaner := func() {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
	return cleaner, nil
}

// attachPlatformProcess 非 Windows 平台占位
func attachPlatformProcess(cmd *exec.Cmd) error {
	return nil
}
