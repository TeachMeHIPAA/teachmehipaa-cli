# tmh documentation

The API reference and guides live at
[docs.teachmehipaa.com/api](https://docs.teachmehipaa.com/api?lang=cli);
`tmh open docs` opens them. The [README](../README.md) covers install and a
first command.

## Reference

- [Command reference](reference/commands/tmh.md): every command, its help
  and its flags, generated from the command tree with `cobra/doc`.
- [`code-samples.json`](reference/code-samples.json): one `tmh` example per
  operationId, for the `cli` tab of the hosted API reference. Generated
  from the SDK IR with `make generate`.

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

- [CONTRIBUTING.md](../CONTRIBUTING.md): setup, the generators, the tests
  and releases.
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
