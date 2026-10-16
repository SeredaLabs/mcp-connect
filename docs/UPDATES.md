# ⬆️ Updates

🌐 **English** · [Українська](UPDATES.uk.md)

[← Overview](../README.md) · [Security](../SECURITY.md)

## 🖥️ In the app

Starting with **0.4.11**, MCP Connect checks public GitHub releases once when it starts. A blue **Update · v…** button appears in the sidebar when a newer version is available. You can also open **ⓘ → About MCP Connect → Check for updates**.

Click the update button to download the installer for your operating system and application architecture. The complete file must match the SHA-256 digest and size published by GitHub before it can run. The standard installer handles replacement and OS approval. Profiles, secrets and client registrations stay in place.

Close active AI-client adapter sessions before installing and restart clients afterward. A new ad-hoc signed macOS build may ask for Keychain permission again. Linux uses the system authorization dialog (`pkexec`) and apt; if these are unavailable, install the verified `.deb` manually with your package manager. The app shows its saved path if launch fails.

**Cancel update** stops the download and removes the partial file. It cannot cancel an installer that has already started.

Versions **0.4.10 and earlier** need one manual update to 0.4.11 or later to gain this feature.

## ⌨️ From a terminal

| System | Command for a standard installation |
|---|---|
| macOS | `"/Applications/MCP Connect.app/Contents/MacOS/mcp-connect" update` |
| Windows PowerShell | `& "$env:LOCALAPPDATA\Programs\MCP Connect\mcp-connect.exe" update` |
| Linux | `mcp-connect update` |

Append **`--check`** to check availability without downloading or installing. Append **`--download`** to save a verified installer and print its path without launching it. These commands do not read vault credentials or call your MCP server. A custom Windows installation needs its actual executable path.

## 📦 Portable copies

Portable copies can check for updates but are not moved or overwritten automatically. Download the new portable archive from the [download page](https://seredalabs.github.io/mcp-connect/), close the app and AI clients, then replace the app at the same permanent location. If you change its path, use **Save & Connect** to repair registrations.

`update --download` explicitly downloads an installer; it does not update a portable archive in place.

## 🔐 Network and trust

Disable **Check for updates on launch** in About to stop automatic checks. Manual checks still work. The preference is stored locally in `updates.json` beside the connection profiles. Verified downloads are stored under the OS user cache in `MCP Connect/updates`; failed or canceled downloads are removed.

Checks contact GitHub's public release API and include the app version in the User-Agent. GitHub receives normal network information, such as your IP address. Connection names, MCP addresses, credentials and business data are never included. There is no separate background service, telemetry or automatic installation. Published Preview releases are included in update checks.

Only HTTPS GitHub release assets and allowlisted GitHub download hosts are accepted. A checksum verifies file integrity against GitHub's release metadata; it is **not** an independent publisher signature. macOS PKG and Windows installers remain unsigned by a publisher. An offline or rate-limited update check does not prevent using the app.
