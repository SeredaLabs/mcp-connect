# 🛡️ Security

🌐 **English** · [Українська](docs/SECURITY.uk.md)

Please report vulnerabilities privately through this repository's **Security → Report a vulnerability**. Do not include real credentials, business data, local profiles or private server addresses in public issues.

MCP Connect stores credentials in the operating system's secure store. Metadata and client registration contain no password, token or API key. Secrets are bound to the exact endpoint and authentication method. Old unbound records require explicit re-entry before use. Unsupported or locked secret stores fail without a plaintext fallback.

HTTPS certificate validation stays enabled. Redirects and cross-origin requests are blocked. API-key headers are allowlisted; token values reject control characters. Diagnostic errors omit secrets and remote response bodies. The adapter does not replay ambiguous business calls after a broken connection.

Client adapters inspect ownership before replacing or removing registrations. Claude Desktop preserves unrelated JSON fields; ChatGPT validates ownership receipts, installation-path containment and managed-directory symlinks. A failed client action does not roll back successful unrelated integrations. Disconnect persists the disabled profile before cleanup, so failed cleanup does not permit new adapter sessions. These controls and their boundaries are described in [integration architecture](docs/INTEGRATIONS.md).

Your OS account, the AI client's tool approvals and the remote server's permissions remain security boundaries. The utility is not a sandbox, a read-only proxy or an authorization service. A process allowed to use your connection can access the tools that your server account permits. Existing client sessions must be stopped to revoke their active adapter processes.

Only the latest release receives fixes. Preview packages are not publisher-signed/notarized. The macOS application has an ad-hoc signature only, not a Developer ID signature. Automated checks and known-vulnerability scans are not a security certification.

Update checks contact only GitHub public releases, without MCP profiles or credentials. Installer downloads require a matching size and SHA-256 digest, HTTPS and allowlisted GitHub hosts. Installation requires an explicit action and OS approval when required. The checksum is not an independent publisher signature. See [updates](docs/UPDATES.md).
