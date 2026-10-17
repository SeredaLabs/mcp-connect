package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func canonicalPath(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return filepath.Clean(platformLongPath(path))
}
func samePath(a, b string) bool {
	a, b = canonicalPath(a), canonicalPath(b)
	if runtime.GOOS == "windows" {
		return strings.EqualFold(a, b)
	}
	return a == b
}

func findCLI(name string, extra []string) string {
	if path, err := exec.LookPath(name); err == nil {
		return path
	}
	home, _ := os.UserHomeDir()
	paths := []string{filepath.Join(home, ".local", "bin", name), filepath.Join(home, ".cargo", "bin", name), filepath.Join("/opt/homebrew/bin", name), filepath.Join("/usr/local/bin", name)}
	if runtime.GOOS == "windows" {
		paths = []string{filepath.Join(home, ".local", "bin", name+".exe"), filepath.Join(home, ".cargo", "bin", name+".exe")}
		paths = append(paths, extra...)
		extra = nil
		if root := os.Getenv("APPDATA"); root != "" {
			paths = append(paths, filepath.Join(root, "npm", name+".cmd"))
		}
		for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramW6432")} {
			if root != "" {
				paths = append(paths, filepath.Join(root, "nodejs", name+".cmd"))
			}
		}
	}
	for _, path := range append(paths, extra...) {
		if f, err := os.Stat(path); err == nil && !f.IsDir() {
			return path
		}
	}
	return ""
}
