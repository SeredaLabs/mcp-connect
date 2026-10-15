# MCP Connect

Made & designed by [SeredaLabs](https://github.com/SeredaLabs).

A compact desktop utility that connects your AI clients to an HTTPS MCP server. Configure a connection once, keep credentials in your operating system's secure store, and let the client start the adapter when needed.

[Download](https://github.com/SeredaLabs/mcp-connect/releases) · [Українська інструкція](docs/INSTALL.uk.md) · [Security](SECURITY.md)

## Platforms

| Platform | Architecture | Package |
|---|---|---|
| macOS 13+ | Apple Silicon / Intel | `.app` in ZIP |
| Windows | AMD64 / ARM64 | `.exe` in ZIP |
| Linux desktop | AMD64 / ARM64 | `.tar.gz` |

Linux builds target Ubuntu 24.04 with a graphical session, compatible OpenGL/Wayland/X11 libraries, D-Bus and an unlocked Secret Service provider such as GNOME Keyring. Other distributions may require additional desktop libraries. Availability of each AI client depends on that client's supported platforms.

## Connect

1. Extract the app into a permanent location.
2. Enter a name and the final HTTPS MCP endpoint.
3. Choose **Login & password**, **Bearer token** or **API key**. Enter credentials locally.
4. Select your installed clients, then **Test Connection** and **Save & Connect**.
5. Restart the selected clients and start a new chat.

Supported integrations: Codex, Claude Code, Claude Desktop and the local-plugin experience in ChatGPT Desktop. Local plugins do not automatically become available in ordinary browser/mobile chats. Codex and Claude Code registration requires their installed client CLI. The utility itself needs no Node, Python, Go runtime, listener or background service.

**Configured** means local registration is saved. **Verified** is the result of an explicit connection check, not continuous monitoring. The connection check enumerates tools without invoking business tools.

## Credentials

Passwords, tokens and keys stay in macOS Keychain, Windows Credential Manager or Linux Secret Service. Client configuration contains only the adapter path and connection ID. The app has no plaintext fallback.

Bearer tokens are entered manually; OAuth sign-in and automatic token renewal are not implemented. API keys support `X-API-Key`, `Api-Key` and `X-Auth-Token`. Your server must support the chosen method. For consistent OS support, the complete credential record is limited to 2560 bytes. Credentials are bound to the server endpoint, method and API-key header.

Before moving or removing the app, disconnect its integrations. After changing its path, use Save & Connect to repair registrations. Restart clients to end existing adapter sessions.

## Build from source

Requirements: Go 1.27.2+, a C compiler and native GUI development libraries.

- macOS: Xcode Command Line Tools.
- Windows AMD64: Git Bash and MSYS2 UCRT64 GCC; ARM64: MSYS2 CLANGARM64 clang (`CC=clang`). A ZIP packager is required.
- Ubuntu: `build-essential libgl1-mesa-dev libegl1-mesa-dev xorg-dev libxkbcommon-dev libwayland-dev wayland-protocols`.

Run `./scripts/build.sh` (or `bash scripts/build.sh` on Windows). Packages are written to `dist/`. Dependency versions are pinned in `go.mod` and `go.sum`.

## Release status

The initial public release is a **preview for developers and early testers**. macOS packages are ad-hoc signed, without Developer ID notarization. Windows packages have no publisher signature. OS publisher warnings may appear. Native automated verification does not establish compatibility with every desktop environment, OS version or AI-client release.

Use a separate server account with minimum permissions. This utility does not change server permissions or filter tools. Application code is MIT licensed; see [LICENSE](LICENSE). Dependency and font licenses are included in [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt) and every package.
