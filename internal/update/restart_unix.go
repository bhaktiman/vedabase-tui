//go:build !windows

package update

import (
	"os"
	"syscall"
)

// Restart replaces the current process with exe, keeping argv as typed.
func Restart(exe string) error {
	return syscall.Exec(exe, os.Args, os.Environ())
}
