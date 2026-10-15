package main

import "syscall"

// Detach only this GUI process. Existing parent terminals stay open, while a
// console allocated by Explorer for the desktop launch is released. Stdio CLI
// modes never call this, preserving inherited MCP pipes and terminal output.
func prepareDesktop() { syscall.NewLazyDLL("kernel32.dll").NewProc("FreeConsole").Call() }
