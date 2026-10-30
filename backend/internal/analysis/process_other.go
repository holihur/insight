//go:build !unix

package analysis

import "os/exec"

func configureProcess(cmd *exec.Cmd) {}
