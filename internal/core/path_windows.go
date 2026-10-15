package core

import (
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// Windows clients may expand 8.3 names (RUNNER~1) to their full names.
// Expand the existing ancestor too when the final executable does not exist yet.
func platformLongPath(path string) string {
	original := path
	var tail []string
	for {
		input, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return original
		}
		buf := make([]uint16, 32768)
		n, err := windows.GetLongPathName(input, &buf[0], uint32(len(buf)))
		if err == nil && n > 0 && n < uint32(len(buf)) {
			path = windows.UTF16ToString(buf[:n])
			if strings.HasPrefix(path, `\\?\UNC\`) {
				path = `\\` + path[8:]
			} else if strings.HasPrefix(path, `\\?\`) {
				path = path[4:]
			}
			for j := len(tail) - 1; j >= 0; j-- {
				path = filepath.Join(path, tail[j])
			}
			return path
		}
		parent := filepath.Dir(path)
		if parent == path || parent == "." {
			return original
		}
		tail = append(tail, filepath.Base(path))
		path = parent
	}
}
