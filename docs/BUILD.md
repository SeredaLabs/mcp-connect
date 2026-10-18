# 🛠️ Build MCP Connect

🌐 **English** · [Українська](BUILD.uk.md)

[← Overview](../README.md) · [🇺🇦 User guide](INSTALL.uk.md) · [Security](../SECURITY.md)

Build a native desktop package with the dependencies pinned in `go.mod` and `go.sum`.

<a id="requirements"></a>

## 📋 Requirements

| Platform | Tools |
|---|---|
| macOS | Go 1.27.2+, Xcode Command Line Tools |
| Windows AMD64 | Go 1.27.2+, Git Bash, MSYS2 UCRT64 GCC |
| Windows ARM64 | Go 1.27.2+, Git Bash, MSYS2 CLANGARM64 clang |
| Ubuntu 24.04 | Go 1.27.2+, C compiler, GUI development libraries |

Windows builds need `7z` or `zip` on PATH. Set `CC` to the full path of the appropriate MSYS2 compiler, so another compiler on PATH cannot take precedence.

For Ubuntu, install:

```sh
sudo apt-get install build-essential dpkg-dev libgl1-mesa-dev libegl1-mesa-dev xorg-dev libxkbcommon-dev libwayland-dev wayland-protocols
```

<a id="build"></a>

## 🛠️ Build

```sh
bash scripts/build.sh
```

Packages are written to `dist/`. They include the executable or application bundle, Ukrainian installation guide, application license and third-party notices.

The [build workflow](https://github.com/SeredaLabs/mcp-connect/actions/workflows/build.yml) builds six native targets, producing a portable archive and an installer for each (12 packages in total), and checks dependency integrity, static analysis and known vulnerabilities. It is not a publisher-signing or notarization service.

Windows packaging generates embedded icon resources with pinned `go-winres v0.3.3`. This build-only tool is downloaded by Go and is not included in the application.

## 📦 Installers

After `bash scripts/build.sh`, run `bash scripts/package-installer.sh` on the target OS. macOS uses built-in `pkgbuild` and `productbuild`; Linux uses `dpkg-deb`; Windows uses Inno Setup 7.1.0 (`ISCC` points to `ISCC.exe`, or `iscc` must be on PATH). The Windows compiler is a build-only tool. CI downloads the official release and checks a pinned SHA-256 digest.

CI installs 0.4.13, upgrades it to the current build on all six targets, checks the installed CLI, and verifies removal and shortcut cleanup on Windows/Linux. These checks do not certify GUI startup or publisher trust. Installers preserve profiles and vault secrets and never register MCP clients during installation.

## Validation and integration changes

The [0.4.13 native run](https://github.com/SeredaLabs/mcp-connect/actions/runs/39031665077) passed on all six targets. These are package, upgrade and installed CLI checks, not manual GUI certification.

After cloning the repository, run:

```sh
go test ./...
go test -race ./...
go vet ./...
go mod verify
```

Tests under `internal/core` and `internal/gui` are versioned. Native CI runs `go test ./... -count=1` on all six targets and the race detector on Linux AMD64. `.local/`, scratch validation and generated packages remain ignored. The 0.4.13 release predates this CI test step; its historical run above covered package/static checks.

CI also runs `go mod tidy -diff` and Staticcheck v0.8.2 with its default checks except ST1005. Application errors are intentionally sentence-cased because the GUI/CLI displays them directly. Correctness, deprecated API and unused-code checks remain enabled. For a reachability audit, run `go run golang.org/x/tools/cmd/deadcode@v0.51.0 ./...` and repeat with `-test`; confirm findings against platform files, callbacks and adapter interfaces before removing code.

Installed-client tests require `ONECMCP_TEST_CLIENT_CLIS=1` and use temporary client homes/configurations. Native-vault tests require `ONECMCP_TEST_NATIVE_VAULT=1` and create/remove synthetic credential records. Both are explicit local opt-ins; client CLIs and an available OS vault are required respectively.

Before changing a client integration, read [the adapter contract and boundaries](INTEGRATIONS.md). Vendor commands, formats, paths and fixtures belong to the affected adapter. The coordinator, GUI and profile format should remain unaffected when the common contract still applies.

## Native GUI startup diagnostics

CI launches the packaged binary with disposable empty configuration and update checks disabled. Linux uses Xvfb/software OpenGL and requires an observable window. macOS uses a native CoreGraphics window probe; Windows uses native window enumeration. A visible window that remains alive passes startup only. A hosted graphics limitation or unconfirmed Windows window is recorded as unavailable/unconfirmed, with a warning and a separate `gui-check-*` report; it is never counted as a GUI pass. Unexpected exits fail the check. The reports include startup logs and do not contain real profiles or credentials.

These probes do not replace interaction testing on physical hardware. Mesa/Xvfb are CI dependencies, not bundled application requirements or shipped software.
