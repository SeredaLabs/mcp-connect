# Client integration adapters

🌐 **English** · [Українська](INTEGRATIONS.uk.md)

[← Overview](../README.md) · [Build and validation](BUILD.md) · [Security](../SECURITY.md)

MCP Connect registers clients through small adapters in `internal/core`. The
coordinator does not parse vendor configuration, construct vendor commands or
choose registration mechanisms. Profile flags and stable IDs remain unchanged.

Here, a client adapter manages local registration. The `mcp-connect serve <profile-ID>` process is the separate runtime transport started by an AI client; its authentication, vault access and recovery behavior are outside this refactor.

## Operation contract

```go
type ClientAdapter interface {
    ID() string
    Read(id string) (Registration, bool, error)
    Set(context.Context, RegistrationRequest) error
    Detect() string
    Settings(id string) (ClientSettings, error)
}
```

The [contract source](../internal/core/client_adapter.go) defines normalized registration data and application intent. No shared type exposes a TOML table, plugin manifest or vendor-specific CLI flag. `Detect` locates a client; `Settings` supplies presentation data; `Read` and `Set` inspect and apply registrations. `Set` must preserve external entries and verify the requested result before reporting success.

## Boundaries

- `client_adapter.go`: the operation contract, ordered registry and shared
  ownership/verification checks. `Registration` is normalized application data;
  vendor serialization tags belong to private structures in each adapter.
- `client_defaults.go`: explicit composition, display names and bindings to the
  existing profile flags. This is where another client is registered.
- `integration.go`, `client_state.go`: lookup, independent application and retry,
  status reporting, and persistence of profile disabling before cleanup.
- `adapter_codex.go`: Codex discovery, npm launcher entrypoint, commands, TOML
  inspection and the OpenAI configuration store shared by desktop plugins.
- `adapter_claude.go`: Claude Code discovery, npm launcher entrypoint, user-scope
  commands, JSON inspection and best-effort restoration after failed replacement.
- `adapter_claude_desktop.go`: desktop discovery and paths, JSON inspection,
  preservation of unrelated fields, concurrent-change detection and atomic writes.
- `adapter_chatgpt.go`: desktop discovery, plugin manifests, marketplace naming,
  ownership receipts, managed-path validation, install/remove commands and result
  parsing. It never parses Codex configuration.
- `client_command.go`, `client_environment.go`, `client_paths.go`: vendor-neutral
  executable invocation, environment and filesystem helpers. Adapters supply their
  own npm entrypoint; Windows launchers are never passed to a shell.

`ClientAdapter` exposes `ID`, `Read`, `Set`, `Detect` and `Settings` operations.
`Set` receives only the profile ID/name, application executable and desired enabled
state. A successful mutation must inspect its resulting registration. `Settings`
returns guidance and actions for the GUI to render; the GUI owns no registration
paths or CLI instructions. `Integrator.ChatGPTSetupURL` remains a thin compatibility
facade. A small optional `ReadShared` operation preserves the existing shared-state
semantics without naming a vendor in the coordinator.

The registry preserves insertion order, rejects duplicate/unknown IDs, and binds
selection to the existing profile fields. Each `Apply` result is independent;
a failed client does not roll back a successful unrelated client. Targeted retry
resolves only the requested IDs.

## Shared OpenAI store

Codex and ChatGPT currently share a configuration store and executable. Explicit
composition supplies the ChatGPT adapter with private backend operations owned
by the Codex adapter: storage location, plugin activation/marketplace ownership
inspection, executable discovery and invocation. Plugin commands and output formats
remain in ChatGPT. Codex receives only ChatGPT's normalized registration reader for
shared-state reporting. Neither dependency goes through the coordinator.

This small internal composition seam is deliberately separate from the shared
client contract. A Codex storage-format migration changes the Codex adapter and
its fixtures. A ChatGPT manifest or installation-command change changes the
ChatGPT adapter and its fixtures. If a vendor stops sharing a backend, update the
affected adapter and explicit composition rather than teaching the coordinator
another mechanism.

## Compatibility and safety

Detection means the app/executable was located, not that its version was certified.
`ClientState.Compatibility` remains `unknown`; a readable registration or successful
CLI invocation must not imply that an unknown upstream version is verified.
Adapters validate known configuration/result shapes and fail per client on invalid
configuration, unsupported commands, changed installation output or failed state
verification. There is no version matrix or compatibility framework.

Ownership remains an absolute application command with exactly `serve <profile-ID>`
arguments. External entries are never replaced or removed. Desktop JSON retains
unrelated values. Plugin receipts, cache containment and symlink checks protect
managed files and directories. Configuration stores no credentials. Profile,
transport, authentication, vault and remote-session formats are unaffected.

## Acceptance scenarios

| Upstream change | Expected implementation change |
| --- | --- |
| Claude Code registration syntax | Claude Code adapter and its fixtures/tests |
| ChatGPT plugin manifest/install command | ChatGPT adapter and its fixtures/tests |
| Codex configuration format | Codex adapter/backend and its fixtures/tests |
| Additional client | New adapter/tests, explicit composition and UI/profile selection exposure |

Isolation applies while the common integration contract remains applicable.

Existing adapters and the coordinator need no new registration dispatch branch
for an additional client. Adding persisted selection for a future client still
requires an explicit storage/UI decision; this refactor does not redesign profiles.

## Local validation

This repository keeps `*_test.go` files local and ignored (see [`.gitignore`](../.gitignore) and the local development instructions in `.local/README.md`). A public clone does not contain this test suite, and native CI runs package/static checks rather than these local tests. Do not force-add them. The adapter-specific test files own
configuration/CLI fixtures; the shared contract suite uses only normalized
operations. ChatGPT's contract fixture substitutes backend operations, so it does
not depend on Codex's TOML representation. Existing end-to-end regression tests
also exercise the real shared store.

Use `go test ./...`, `go test -race ./...`, `go vet ./...`, and `go mod verify`.
Installed CLI tests require `ONECMCP_TEST_CLIENT_CLIS=1`; they override client
homes/configuration with temporary directories and must not use real profiles.
Cross-compilation verifies Go source compatibility only, not native desktop startup,
installation or behavior on another operating system.

## Adding a client

1. Implement the operation contract in a focused adapter and keep its formats, commands, detection and compatibility checks there.
2. Add adapter-specific fixtures and run the shared behavioral contract suite.
3. Add an explicit binding in `client_defaults.go`, including its stable ID, name and selection predicate; expose the choice in the GUI. If the choice needs persistence, decide its profile field explicitly.
4. Verify independent apply/retry failures and preservation of existing client registrations. Do not add a vendor dispatch branch to `Integrator`.

For the current release, [0.4.13 native CI](https://github.com/SeredaLabs/mcp-connect/actions/runs/39031665077) passed all six package/upgrade targets. Local tests separately exercised the contract suite, installed client CLIs, GUI and macOS vault, including the race detector. Unknown upstream versions still remain `unknown`.
