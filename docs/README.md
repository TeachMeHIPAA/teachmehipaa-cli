# Documentation

Documentation for `tmh`, the TeachMeHIPAA CLI for the Developer API. New
here? Start with [Getting started](guides/getting-started.md).

## Guides

1. [Getting started](guides/getting-started.md): install, Sandbox
   credentials, first calls, inviting a Learner, handling failures.
2. [Authentication and profiles](guides/authentication-and-profiles.md):
   `tmh login`, the `sandbox` and `live` profiles and their Environments,
   credentials from the environment for CI, Scopes, the Organization binding
   and public lookups without credentials.
3. [Commands](guides/commands.md): resources and methods, arguments and
   flags, `-d` and `--data`, ETags and `--if-match`, idempotency keys,
   retries and raw requests.
4. [Output and scripting](guides/output-and-scripting.md): JSON on stdout,
   `jq`, pagination with `--all` and `--ndjson`, exit codes in scripts and
   `--verbose`.
5. [Webhooks and `events tail`](guides/webhooks-and-events-tail.md): the
   Event log, forwarding Events to a local handler, the local Signing secret
   and signature, Webhook subscriptions and Test deliveries.
6. [Operations and files](guides/operations-and-files.md): `--wait`, CSV
   exports, Certificate PDFs, CSV imports and batch changes.
7. [Exit codes and errors](guides/exit-codes.md): every exit code, how
   errors print, request ids.
8. [Configuration and privacy](guides/configuration-and-privacy.md):
   environment variables, global flags, config and state files, what `tmh`
   connects to, the version check and shell completion.

## Reference

- [Command reference](reference/commands/tmh.md): every command, its help
  and its flags, generated from `tmh`'s command tree with `cobra/doc`.
- [Resource reference](reference/resources.md): every Developer API
  operation with the command that calls it, its HTTP route, Scopes, retry,
  idempotency and conditional behavior, and what it prints. Generated from
  the Developer API's SDK IR.

## Examples

Shell scripts in [`examples/`](../examples/README.md), each reading
`TMH_CLIENT_ID` and `TMH_CLIENT_SECRET` and targeting the Sandbox:

- [`quickstart.sh`](../examples/quickstart.sh): the Compliance summary and a
  few Learners.
- [`sync-roster.sh`](../examples/sync-roster.sh): sync a roster CSV with
  `learners upsert-by-external-id`.
- [`export-certificates.sh`](../examples/export-certificates.sh): export
  Certificates to CSV with `--wait -o`.
- [`webhook-dev-loop.sh`](../examples/webhook-dev-loop.sh): forward Events
  to a local webhook handler with `events tail --forward-to`.

## Contributing and design records

These explain how `tmh` is built and why. You don't need them to use it.

- [CONTRIBUTING.md](../CONTRIBUTING.md): setup, the generators, tests,
  releases and how the documentation is built and kept in sync.
- [`adr/`](adr/0001-generated-surface-from-shared-ir.md): architecture
  decision records, such as
  [generated surface from a shared IR](adr/0001-generated-surface-from-shared-ir.md),
  [the Organization bound to the client](adr/0003-organization-bound-to-client.md),
  [webhook verification needs the path](adr/0005-webhook-verification-needs-the-path.md),
  [no telemetry, no body logging](adr/0007-no-telemetry-no-body-logging.md) and
  [a standalone Go binary with `events tail`](adr/0008-go-standalone-events-tail.md).
  Superseded records stay, marked as such.
- [`design/`](design/grilling-2026-11-15.md): the design review that settled
  the CLI's and SDKs' interface.
- [`research/`](research/stripe-cli.md): research into the Stripe CLI, whose
  conventions `tmh` follows where they fit.
- [`agents/`](agents/domain.md): conventions for coding agents working on
  this repository: domain docs, the issue tracker and triage labels.
- [GLOSSARY.md](../GLOSSARY.md): the terms these docs use, such as Learner,
  Enrollment and Delivery.
