package main

import (
	"bufio"
	"errors"
	"os"
	"runtime"
	"strings"
	"syscall"
)

func mounted(path string) bool {
	if runtime.GOOS != "linux" {
		return true
	}
	f, err := os.Open("/proc/mounts")
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		v := strings.Fields(sc.Text())
		if len(v) > 1 {
			p := strings.NewReplacer(`\040`, " ", `\011`, "\t", `\012`, "\n", `\134`, `\`).Replace(v[1])
			if p == path {
				return true
			}
		}
	}
	return false
}
func drop(c config) error {
	if os.Geteuid() != 0 {
		if os.Geteuid() != c.UID || os.Getegid() != c.GID {
			return errors.New("run as configured uid/gid or start as root to bind and drop privileges")
		}
		return nil
	}
	if err := syscall.Setgroups([]int{}); err != nil {
		return err
	}
	if err := syscall.Setgid(c.GID); err != nil {
		return err
	}
	return syscall.Setuid(c.UID)
}
