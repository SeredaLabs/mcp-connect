# 🔑 Authentication

🌐 **English** · [Українська](AUTHENTICATION.uk.md)

[← Overview](../README.md) · [🇺🇦 User guide](INSTALL.uk.md) · [Security](../SECURITY.md)

Choose the method supported by your MCP server. The utility does not enable authentication methods on the server itself.

| Method | Enter | Sent to the server |
|---|---|---|
| Login & password | Login and hidden password | HTTP Basic authentication over HTTPS |
| Bearer token | Hidden token without the `Bearer` prefix | `Authorization: Bearer …` |
| API key | Header selection and hidden key | `X-API-Key`, `Api-Key` or `X-Auth-Token` |

<a id="storage"></a>

## 🔐 Storage

Secrets are stored in macOS Keychain, Windows Credential Manager or Linux Secret Service. Profiles contain connection metadata, and client registration contains the adapter path and profile ID. Credentials are not copied into client configuration.

An unavailable or locked vault causes an error; there is no plaintext fallback. The complete credential record, including its endpoint binding, is limited to **2560 bytes** for consistent Windows support. Very long tokens may not fit.

<a id="editing-a-connection"></a>

## ✏️ Editing a connection

Empty credential fields reuse a saved secret only when its endpoint, authentication method and API-key header match. Changing these requires re-entry. Older records without an endpoint binding must be explicitly re-entered and saved.

Changing only the profile name or client choices uses **Save & Apply** without reading vault credentials or contacting the MCP server. Endpoint or credential changes are checked before saving.

On macOS, Keychain access depends on the executable and its signing identity. A new ad-hoc signed build may ask for permission again.

<a id="tokens-and-permissions"></a>

## 🔑 Tokens and permissions

Tokens and keys are entered manually. OAuth sign-in, automatic token renewal and arbitrary custom headers are not implemented.

A `401` response means authentication failed; a `403` response means access was denied. Check expiry, server permissions and token scope. The app cannot grant server permissions.

Disconnect disables the profile before removing local registrations and keeps the saved secret. If cleanup fails, the profile stays disabled and cleanup can be retried. Delete also removes the profile and secret. Restart clients to stop already running adapter sessions.
