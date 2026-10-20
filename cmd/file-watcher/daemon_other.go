//go:build !windows

package main

import "os/exec"

func detachDaemonProcess(_ *exec.Cmd) {}
