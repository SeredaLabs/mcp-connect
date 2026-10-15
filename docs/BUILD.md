# Build MCP Connect

[← Overview](../README.md) · [🇺🇦 User guide](INSTALL.uk.md) · [Security](../SECURITY.md)

Build a native desktop package with the dependencies pinned in `go.mod` and `go.sum`.

## Requirements

| Platform | Tools |
|---|---|
| macOS | Go 1.27.2+, Xcode Command Line Tools |
| Windows AMD64 | Go 1.27.2+, Git Bash, MSYS2 UCRT64 GCC |
| Windows ARM64 | Go 1.27.2+, Git Bash, MSYS2 CLANGARM64 clang |
| Ubuntu 24.04 | Go 1.27.2+, C compiler, GUI development libraries |

Windows builds need `7z` or `zip` on PATH. Set `CC` to the full path of the appropriate MSYS2 compiler, so another compiler on PATH cannot take precedence.

For Ubuntu, install:

```sh
sudo apt-get install build-essential libgl1-mesa-dev libegl1-mesa-dev xorg-dev libxkbcommon-dev libwayland-dev wayland-protocols
```

## Build

```sh
bash scripts/build.sh
```

Packages are written to `dist/`. They include the executable or application bundle, Ukrainian installation guide, application license and third-party notices.

The [build workflow](https://github.com/SeredaLabs/mcp-connect/actions/workflows/build.yml) compiles six native targets and checks dependency integrity, static analysis and known vulnerabilities. It is not a publisher-signing or notarization service.
