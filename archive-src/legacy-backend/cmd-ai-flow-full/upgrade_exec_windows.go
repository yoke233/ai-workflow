//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
)

func execUpgradeBinary(binaryPath string, argv []string, env []string) error {
	args := []string{}
	if len(argv) > 1 {
		args = append(args, argv[1:]...)
	}
	cmd := exec.Command(binaryPath, args...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start upgraded binary: %w", err)
	}
	return nil
}
