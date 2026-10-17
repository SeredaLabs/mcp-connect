package core

import (
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"testing"
)

func TestWindowsShortAndLongPaths(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Folder With Long Name")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	input, err := windows.UTF16PtrFromString(dir)
	if err != nil {
		t.Fatal(err)
	}
	buf := make([]uint16, 32768)
	n, err := windows.GetLongPathName(input, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	long := windows.UTF16ToString(buf[:n])
	if !samePath(dir, long) {
		t.Fatal("Temp directory aliases do not match")
	}
	n, err = windows.GetShortPathName(input, &buf[0], uint32(len(buf)))
	if err != nil {
		t.Fatal(err)
	}
	short := windows.UTF16ToString(buf[:n])
	if !samePath(short, long) || !samePath(filepath.Join(short, "Future App", "connect.exe"), filepath.Join(long, "Future App", "connect.exe")) {
		t.Fatal("Short and long path aliases do not match")
	}
	if samePath(short, filepath.Join(long, "unrelated")) {
		t.Fatal("Different directories match")
	}
}
