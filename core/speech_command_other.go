//go:build !windows

package core

import "os/exec"

func configureSpeechCommand(_ *exec.Cmd) {}
