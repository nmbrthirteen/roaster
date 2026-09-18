//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// createNoWindow keeps the server's console off the screen. roaster.exe is a
// console program on purpose, so that -set-token and the designer still print
// when someone runs them from a terminal, but the stand starts it from a window
// application and Windows would give it a console of its own in front of the
// kiosk.
const createNoWindow = 0x08000000

func quiet(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
