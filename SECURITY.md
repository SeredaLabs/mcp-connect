# 🛡️ Security

🌐 **English** · [Українська](docs/SECURITY.uk.md)

Please report vulnerabilities privately through this repository's **Security → Report a vulnerability**. Do not include real credentials, business data, local profiles or private server addresses in public issues.

MCP Connect stores credentials in the operating system's secure store. Metadata and client registration contain no password, token or API key. Secrets are bound to the exact endpoint and authentication method. Old unbound records require explicit re-entry before use. Unsupported or locked secret stores fail without a plaintext fallback.

HTTPS certificate validation stays enabled. Redirects and cross-origin requests are blocked. API-key headers are allowlisted; token values reject control characters. Diagnostic errors omit secrets and remote response bodies. The adapter does not replay ambiguous business calls after a broken connection.

Your OS account, the AI client's tool approvals and the remote server's permissions remain security boundaries. The utility is not a sandbox, a read-only proxy or an authorization service. A process allowed to use your connection can access the tools that your server account permits. Existing client sessions must be stopped to revoke their active adapter processes.

Only the latest release receives fixes. Preview packages are not publisher-signed/notarized. Automated checks and known-vulnerability scans are not a security certification.
