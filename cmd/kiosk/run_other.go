//go:build !windows

package main

import "os/exec"

// quiet has nothing to hide away from Windows.
func quiet(cmd *exec.Cmd) {}
