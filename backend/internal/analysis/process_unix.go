//go:build unix

package analysis

import (
	"os/exec"
	"syscall"
)

// configureProcess makes cancellation kill the whole git process group,
// including helpers such as git-remote-https that keep pipes open.
func configureProcess(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
