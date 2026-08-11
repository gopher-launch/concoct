//go:build windows

package execution

import (
	"os/exec"
	"strconv"
)

func configureProcess(_ *exec.Cmd) {}
func terminateProcess(cmd *exec.Cmd) {
	taskkill(cmd, false)
}
func killProcess(cmd *exec.Cmd) {
	taskkill(cmd, true)
}
func taskkill(cmd *exec.Cmd, force bool) {
	if cmd.Process == nil {
		return
	}
	args := []string{"/PID", strconv.Itoa(cmd.Process.Pid), "/T"}
	if force {
		args = append(args, "/F")
	}
	_ = exec.Command("taskkill", args...).Run()
}
