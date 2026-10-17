"""Observe a native window using an empty disposable profile, never real secrets.

Exit 0: visible window remained alive. Exit 77: GUI environment unavailable or
unconfirmed on a hosted desktop. Exit 1: failure in a prepared GUI environment.
No business tools or client registration actions are performed.
"""
import argparse
import ctypes
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time

parser = argparse.ArgumentParser()
parser.add_argument("--platform", choices=("darwin", "linux", "windows"), required=True)
parser.add_argument("--arch", choices=("amd64", "arm64"), required=True)
parser.add_argument("--output", required=True)
args = parser.parse_args()
output = Path(args.output).resolve()
output.mkdir(parents=True, exist_ok=True)
base = Path("dist") / f"{args.platform}-{args.arch}"
filename = {"darwin": "MCP Connect.app/Contents/MacOS/mcp-connect", "linux": "mcp-connect", "windows": "mcp-connect.exe"}[args.platform]
exe = (base / filename).resolve()
result = {"platform": args.platform, "arch": args.arch, "status": "failed", "detail": "Window not observed", "software_rendering": args.platform == "linux" and os.getenv("LIBGL_ALWAYS_SOFTWARE") == "1"}
observed = set()


def visible_window(pid):
    if args.platform == "linux":
        found = subprocess.run(["xdotool", "search", "--onlyvisible", "--pid", str(pid)], capture_output=True)
        return found.returncode == 0 and bool(found.stdout.strip())
    if args.platform == "darwin":
        probe = Path(os.environ["RUNNER_TEMP"]) / "gui-window"
        return subprocess.run([str(probe), str(pid)], capture_output=True).returncode == 0
    user32 = ctypes.WinDLL("user32", use_last_error=True)
    callback_type = ctypes.WINFUNCTYPE(ctypes.c_int32, ctypes.c_void_p, ctypes.c_void_p)
    found = []

    @callback_type
    def inspect(hwnd, unused):
        owner = ctypes.c_ulong()
        user32.GetWindowThreadProcessId(ctypes.c_void_p(hwnd), ctypes.byref(owner))
        if owner.value == pid:
            title = ctypes.create_unicode_buffer(256)
            user32.GetWindowTextW(ctypes.c_void_p(hwnd), title, 256)
            visible = bool(user32.IsWindowVisible(ctypes.c_void_p(hwnd)))
            observed.add((title.value, visible))
            if visible and title.value == "MCP Connect":
                found.append(hwnd)
        return True

    user32.EnumWindows(inspect, None)
    return bool(found)


code = 1
with tempfile.TemporaryDirectory(prefix="mcp-connect-gui-") as temporary:
    home = Path(temporary)
    env = dict(os.environ)
    env.update(HOME=str(home), XDG_CONFIG_HOME=str(home / "config"), APPDATA=str(home / "config"), CODEX_HOME=str(home / ".codex"), CLAUDE_CONFIG_DIR=str(home / ".claude"))
    env.pop("WAYLAND_DISPLAY", None)
    config = home / "Library/Application Support" if args.platform == "darwin" else home / "config"
    preferences = config / "MCP Connect/updates.json"
    preferences.parent.mkdir(parents=True)
    preferences.write_text('{"check_on_launch": false}\n')
    with (output / "startup.log").open("wb") as log:
        process = subprocess.Popen([str(exe)], env=env, stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT)
        try:
            deadline = time.monotonic() + 45
            while time.monotonic() < deadline and process.poll() is None:
                if visible_window(process.pid):
                    time.sleep(3)
                    if process.poll() is None and visible_window(process.pid):
                        result.update(status="passed", detail="Native window visible and process alive with empty disposable profile")
                        code = 0
                    break
                time.sleep(0.5)
            if code != 0:
                log.flush()
                details = (output / "startup.log").read_text(errors="replace")
                result.update(exit_code=process.poll(), observed_windows=sorted(observed), detail=details[-6000:] or "Native window did not become visible")
                # These are diagnostic results, never a passing window check.
                if args.platform == "darwin" and any(s in details for s in ("NSGL", "Failed to find a suitable pixel format")):
                    result.update(status="unavailable", detail="Hosted macOS graphics context unavailable; physical desktop check required")
                    code = 77
                elif args.platform == "windows" and (process.poll() is None or any(s in details for s in ("WGL", "OpenGL", "GLFW"))):
                    result.update(status="unconfirmed", detail="Hosted Windows graphics/window startup unavailable or unconfirmed; native desktop check required", diagnostics=details[-6000:])
                    code = 77
        finally:
            if process.poll() is None:
                process.terminate()
                try:
                    process.wait(timeout=10)
                except subprocess.TimeoutExpired:
                    process.kill()
                    process.wait(timeout=10)
(output / "result.json").write_text(json.dumps(result, indent=2) + "\n")
print(json.dumps(result))
summary = os.getenv("GITHUB_STEP_SUMMARY")
if summary:
    with open(summary, "a", encoding="utf-8") as f:
        f.write(f"\nGUI {args.platform}/{args.arch}: **{result['status']}** — {result['detail']}\n")
raise SystemExit(code)
