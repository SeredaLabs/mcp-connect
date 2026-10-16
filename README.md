<h1>
  MCP Connect
  <a href="https://github.com/SeredaLabs/mcp-connect/blob/main/docs/INSTALL.uk.md"><img align="right" src="assets/languages/uk-inactive.svg" alt="🇺🇦 Українська" width="148" height="32"></a>
  <a href="https://github.com/SeredaLabs/mcp-connect"><img align="right" src="assets/languages/en-active.svg" alt="🇬🇧 English" width="148" height="32"></a>
</h1>

[![Build](https://github.com/SeredaLabs/mcp-connect/actions/workflows/build.yml/badge.svg?branch=main)](https://github.com/SeredaLabs/mcp-connect/actions/workflows/build.yml)
[![Preview](https://img.shields.io/badge/release-0.4.7%20preview-f59e0b)](https://github.com/SeredaLabs/mcp-connect/releases)
[![Platforms](https://img.shields.io/badge/platforms-macOS%20%7C%20Windows%20%7C%20Linux-blue)](#platforms)
[![License](https://img.shields.io/badge/license-MIT-purple)](LICENSE)

### 🔌 Your MCP servers, connected to the apps you use.

MCP Connect is a compact desktop utility for connecting AI clients to HTTPS MCP servers. Enter your server and credentials, choose your apps, and connect. Secrets stay in your operating system's secure store; the client starts the adapter when needed.

**Use the app:** [Download](https://github.com/SeredaLabs/mcp-connect/releases) · [Setup](#get-connected) · [Authentication](docs/AUTHENTICATION.md)

**Project:** [Security](SECURITY.md) · [Build from source](docs/BUILD.md)

<a id="see-it-in-action"></a>

## 🎬 See it in action

![MCP Connect — connection profile, three authentication methods and AI client selection](assets/demo/mcp-connect-preview.gif)

Connection profiles → login & password / Bearer token / API key → AI client selection. The GIF loops automatically; server addresses and credential fields are blurred. Demonstration data only, without a live server connection.

[▶ Watch or download the full MP4 demo](assets/demo/mcp-connect-demo.mp4)

<a id="why-mcp-connect"></a>

## ✨ Why MCP Connect?

- **One compact window.** Connection profiles, authentication and client status in one place.
- **Three authentication methods.** Login & password, Bearer token or API key.
- **System credential storage.** macOS Keychain, Windows Credential Manager or Linux Secret Service.
- **No background service.** The adapter runs on demand; the packaged utility needs no Go, Node or Python runtime.

<a id="get-connected"></a>

## 🔌 Get connected

1. Download the installer for your OS and processor from [Releases](https://github.com/SeredaLabs/mcp-connect/releases).
2. Install it and open **MCP Connect** from Applications or your application menu. For a portable archive, extract it to a permanent location first.
3. Enter a connection name and the final **HTTPS MCP endpoint**.
4. Choose an authentication method and enter your credentials locally.
5. Select your installed clients, then **Test Connection** and **Save & Connect**.
6. Restart the clients and start a new chat.

**Test Connection** checks the MCP handshake and lists tools without invoking business tools. Click a client status badge to open its settings or setup guidance.

<a id="supported-clients"></a>

## 🤖 Supported clients

| Provider | Client | Connection |
|---|---|---|
| OpenAI | Codex | Local MCP registration through its installed CLI |
| OpenAI | ChatGPT Desktop | Local plugin in a chat with local execution |
| Anthropic | Claude Desktop | Local desktop MCP configuration |
| Anthropic | Claude Code | Local MCP registration through its installed CLI |

Each client has its own OS support and availability. The ChatGPT integration needs a compatible bundled or installed Codex CLI. A local plugin does not automatically appear in ordinary browser or mobile chats.

<a id="platforms"></a>

## 💻 Platforms

| Platform | Processor | Package |
|---|---|---|
| macOS 13+ | Apple Silicon / Intel | `.pkg` installer or `.app` in ZIP |
| Windows | AMD64 / ARM64 | `-setup.exe` installer or portable ZIP |
| Linux desktop | AMD64 / ARM64 | `.deb` installer or `.tar.gz` |

A graphical session and OpenGL 2.1 compatible graphics drivers are required. Linux packages target Ubuntu 24.04 and need OpenGL/Wayland/X11 libraries, D-Bus and an unlocked Secret Service provider such as GNOME Keyring. Other distributions may need additional desktop libraries.

### 📦 Standard installation

| Installer | Destination | Shortcuts and removal |
|---|---|---|
| macOS `.pkg` | `/Applications/MCP Connect.app` | Applications; remove the app after disconnecting integrations |
| Windows `-setup.exe` | `%LOCALAPPDATA%\Programs\MCP Connect` | Start menu, optional desktop shortcut, Windows Installed apps |
| Linux `.deb` | `/opt/mcp-connect` | Application menu and `mcp-connect` command; remove through the package manager |

Install before setting up connections. macOS and Linux installers require administrator approval; Windows installs for the current user without elevation. Installers copy the app and shortcuts; they do not enter credentials, configure MCP clients, or install AI clients.

On Ubuntu 24.04 or a compatible system, open the `.deb` in your software installer or run `sudo apt install ./mcp-connect-linux-amd64.deb` (use the ARM64 file on ARM systems). Other Linux distributions can use the archive with compatible desktop libraries.

Before updating or uninstalling, close the app and MCP client sessions. Disconnect integrations before uninstalling. Profiles and vault secrets are preserved; use **Delete** in the app to remove a saved connection and its secret. Moving from a portable copy requires **Save & Connect** once from the installed copy to update adapter paths.

Installers are not publisher-signed: macOS `.pkg` files have no Developer ID Installer signature/notarization, and Windows installers have no Authenticode publisher signature. OS approval warnings can still appear. The macOS application retains its ad-hoc signature.


### 🧳 Portable downloads

Archives are named `mcp-connect-<platform>-<architecture>-portable.zip` (macOS/Windows) or `-portable.tar.gz` (Linux). Choose `portable` for running without an installer; choose `.pkg`, `-setup.exe` or `.deb` for standard installation. In download filenames, `darwin` means macOS.

Portable describes the application files: profiles and credentials still use this computer's configuration folder and OS vault. They do not travel with the archive or a USB drive. Keep the extracted app in a permanent location before connecting clients.

### 🖼️ Application icons

The logo is embedded in the macOS `.app` and Windows `.exe`. The Linux `.deb` installs the icon and launcher automatically. For a **portable Linux archive**, install the included files before configuring connections:

```sh
install -Dm755 mcp-connect "$HOME/.local/bin/mcp-connect"
install -Dm644 icon.png "$HOME/.local/share/icons/hicolor/512x512/apps/mcp-connect.png"
install -Dm644 mcp-connect.desktop "$HOME/.local/share/applications/mcp-connect.desktop"
```

`~/.local/bin` must be on the desktop session's `PATH`. File managers may still use a generic icon for the standalone Linux executable; the launcher and application window use the logo.

<a id="understand-the-indicators"></a>

## 🚦 Understand the indicators

| Indicator | Meaning |
|---|---|
| **Configured** · blue | Local registration is saved |
| **Verified** · green | The last explicit server check succeeded |
| **Test required** · amber | Settings changed or need checking |
| **Disabled / Not configured** · grey | The profile or client integration is inactive |

Indicators are not continuous monitoring. Restart clients to end existing adapter sessions after disconnecting.

<a id="credentials-and-security"></a>

## 🔐 Credentials and security

Passwords, tokens and API keys stay in your system vault. Client configuration contains only the adapter path and connection ID. There is no plaintext fallback.

Credentials are bound to the endpoint and authentication method. Changing either requires entering credentials again. OAuth sign-in and automatic token renewal are not implemented. See [Authentication](docs/AUTHENTICATION.md) for supported headers and limits.

Use a separate server account with minimum permissions. MCP Connect does not change server permissions or restrict the tools your account can invoke. Report vulnerabilities privately through [Security → Report a vulnerability](https://github.com/SeredaLabs/mcp-connect/security/advisories/new).

<a id="preview-release"></a>

## 🧪 Preview release

The first public release is intended for developers and early testers. macOS packages are ad-hoc signed without Developer ID notarization; Windows packages have no publisher signature. OS publisher warnings may appear.

Native builds and automated checks do not establish compatibility with every desktop environment or AI-client version. The Apple Silicon interface has been checked on a physical Mac; Linux on both architectures and Windows AMD64 have passed graphical startup checks (Windows used CI-only software OpenGL). Windows ARM64 is experimental: its adapter and credential store passed checks, but its graphical startup and a physical Intel Mac still need confirmation.

<a id="license-and-author"></a>

## 📄 License and author

Application code is [MIT licensed](LICENSE). Dependency and font licenses are included in [THIRD_PARTY_NOTICES.txt](THIRD_PARTY_NOTICES.txt) and every package.

Made & designed by **[SeredaLabs](https://github.com/SeredaLabs)**.
