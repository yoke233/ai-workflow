//go:build !windows

package main

import "syscall"

func execUpgradeBinary(binaryPath string, argv []string, env []string) error {
	return syscall.Exec(binaryPath, argv, env)
}
