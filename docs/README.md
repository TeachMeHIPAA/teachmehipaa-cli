# tmh documentation

`tmh` is the TeachMeHIPAA Developer API on the command line. The topic pages
below open with a synopsis of the commands they cover; prose follows. New
here: [install](topics/install.md), then [login](topics/login.md), then
[recipes](topics/recipes.md).

## Topics

| Page | Covers |
|---|---|
| [install](topics/install.md) | `go install`, `make build`, release channels, checksum verification, Docker, `tmh completion` |
| [login](topics/login.md) | `tmh login`, `logout`, `whoami`; the `sandbox` and `live` profiles; CI credentials; Scopes; public operations |
| [requests](topics/requests.md) | Positional ids, flags, `-d` and `--data`, `--if-match`, idempotency keys, retries, raw `get`/`post`/`put`/`patch`/`delete`, `tmh open` |
| [output](topics/output.md) | JSON on stdout, pages, `--all`, `--ndjson`, `--verbose`, `--show-headers` |
| [operations](topics/operations.md) | `--wait`, CSV exports with `-o`, Certificate PDFs, CSV imports with `--preview`, batch changes |
| [events](topics/events.md) | `events list`, `events tail --forward-to`, `events resend`, the local Signing secret and signature, Webhook subscriptions, Test deliveries |
| [exit-codes](topics/exit-codes.md) | Exit codes 0 to 7 and 130, error output, request ids, `case $?` |
| [environment](topics/environment.md) | Environment variables, global flags, `config.toml` and `state.toml`, privacy, the version check |
| [recipes](topics/recipes.md) | jq pipelines, CSV files, `while read` loops over pages, tailing Events, CI gates |

## Reference

- [Command reference](reference/commands/tmh.md): every command, its help
  and its flags, generated from the command tree with `cobra/doc`.
- [Resource reference](reference/resources.md): every Developer API
  operation with its command, HTTP route, Scopes, retries, idempotency,
  conditional behavior and an example, generated from the SDK IR.
- [`code-samples.json`](reference/code-samples.json): the same examples, one
  per operationId, for the `cli` tab of the API reference. Generated from
  the SDK IR with `make generate`.

## Examples

Scripts in [`examples/`](../examples/README.md). Each reads
`TMH_CLIENT_ID` and `TMH_CLIENT_SECRET` and targets the Sandbox:

| Script | Does |
|---|---|
| [`quickstart.sh`](../examples/quickstart.sh) | Prints the Compliance summary and five Learners |
| [`sync-roster.sh`](../examples/sync-roster.sh) | Syncs a roster CSV with `learners upsert-by-external-id` |
| [`export-certificates.sh`](../examples/export-certificates.sh) | Exports Certificates to CSV with `--wait -o`, optionally with PDFs |
| [`webhook-dev-loop.sh`](../examples/webhook-dev-loop.sh) | Forwards Events to a local handler with `events tail --forward-to` |

## Design records

These explain how `tmh` is built and why. Using it doesn't need them.

- [CONTRIBUTING.md](../CONTRIBUTING.md): setup, the generators, the tests,
  releases, and how the docs stay in sync with the code.
- [`adr/`](adr/0001-generated-surface-from-shared-ir.md): architecture
  decision records, such as
  [generated surface from a shared IR](adr/0001-generated-surface-from-shared-ir.md),
  [the Organization bound to the client](adr/0003-organization-bound-to-client.md),
  [webhook verification needs the path](adr/0005-webhook-verification-needs-the-path.md),
  [no telemetry, no body logging](adr/0007-no-telemetry-no-body-logging.md) and
  [a standalone Go binary with `events tail`](adr/0008-go-standalone-events-tail.md).
  Superseded records stay, marked as such.
- [`design/`](design/grilling-2026-11-15.md): the design review that settled
  the interface of the CLI and the SDKs.
- [`research/`](research/stripe-cli.md): research into the Stripe CLI, whose
  conventions `tmh` follows where they fit.
- [`agents/`](agents/domain.md): conventions for coding agents in this
  repository: domain docs, the issue tracker, triage labels.
- [GLOSSARY.md](../GLOSSARY.md): the terms these docs use, such as Learner,
  Enrollment and Delivery.
