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

CI checks installation and the installed CLI on all six targets, plus removal and shortcut cleanup on Windows/Linux. These checks do not certify GUI startup or publisher trust. Installers preserve profiles and vault secrets and never register MCP clients during installation.
