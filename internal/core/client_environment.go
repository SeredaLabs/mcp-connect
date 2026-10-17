package core

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Finder launches GUI applications with a minimal PATH. npm client launchers
// use /usr/bin/env node, so make the discovered client's directory available
// to its own dependencies without invoking a shell or reading shell profiles.
func clientEnvironment(bin string) []string {
	env := os.Environ()
	key := "PATH="
	dirs := []string{filepath.Dir(bin), os.Getenv("PATH")}
	if runtime.GOOS != "windows" {
		dirs = append(dirs, "/opt/homebrew/bin", "/usr/local/bin")
	} else {
		for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramW6432")} {
			if root != "" {
				dirs = append(dirs, filepath.Join(root, "nodejs"))
			}
		}
	}
	path := strings.Join(dirs, string(os.PathListSeparator))
	for n, v := range env {
		if strings.HasPrefix(strings.ToUpper(v), key) {
			env[n] = "PATH=" + path
			return env
		}
	}
	return append(env, "PATH="+path)
}
