//go:build !windows

package core

func platformLongPath(path string) string { return path }
