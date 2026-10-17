package core

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Invoke known npm entrypoints with Node directly. This supports Windows .cmd
// launchers without cmd.exe, shell expansion or string-built command lines.
// Native client executables do not need Node.
func clientCommand(bin string, args []string, relative string) (string, []string, error) {
	ext := strings.ToLower(filepath.Ext(bin))
	if ext != ".cmd" && ext != ".bat" {
		return bin, args, nil
	}
	if relative == "" {
		return "", nil, errors.New("Unsupported client launcher; use the native client executable")
	}
	entry := ""
	roots := []string{filepath.Join(filepath.Dir(bin), "node_modules")}
	if filepath.Base(filepath.Dir(bin)) == ".bin" {
		roots = append(roots, filepath.Dir(filepath.Dir(bin)))
	}
	for _, root := range roots {
		p := filepath.Join(root, relative)
		if f, e := os.Stat(p); e == nil && !f.IsDir() {
			entry = p
			break
		}
	}
	if entry == "" {
		return "", nil, errors.New("Cannot locate the installed npm client entrypoint; reinstall the client or use its native executable")
	}
	node := ""
	candidates := []string{filepath.Join(filepath.Dir(bin), "node.exe")}
	if p, e := exec.LookPath("node"); e == nil {
		candidates = append(candidates, p)
	}
	for _, root := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramW6432")} {
		if root != "" {
			candidates = append(candidates, filepath.Join(root, "nodejs", "node.exe"))
		}
	}
	if root := os.Getenv("NVM_SYMLINK"); root != "" {
		candidates = append(candidates, filepath.Join(root, "node.exe"))
	}
	for _, p := range candidates {
		if f, e := os.Stat(p); e == nil && !f.IsDir() {
			node = p
			break
		}
	}
	if node == "" {
		return "", nil, errors.New("The npm-installed client needs its Node executable; repair the client installation or use the native client")
	}
	return node, append([]string{entry}, args...), nil
}
