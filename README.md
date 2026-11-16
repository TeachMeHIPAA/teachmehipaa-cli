# tmh: the TeachMeHIPAA CLI

Work with the TeachMeHIPAA Developer API from your terminal.

[![CI](https://github.com/TeachMeHIPAA/teachmehipaa-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/TeachMeHIPAA/teachmehipaa-cli/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

> [!IMPORTANT]
> **Status: pre-release.** The TeachMeHIPAA Developer API isn't generally
> available yet, and `tmh` has no published release: Homebrew, Scoop, the
> Linux packages and the Docker image don't exist until the first one. Until
> then, install it from source as shown below. Commands and output may still
> change before 1.0.

`tmh` lets you explore the API, script admin tasks such as roster syncs and
Certificate exports, and test your webhook handler against real Events
without a public URL. It is a single static binary with no runtime
dependencies, and it handles Access tokens, the Organization binding,
retries, idempotency keys, pagination and Operations for you.

## Install

With Go 1.24 or newer, install the latest source from GitHub:

<!-- doctest: illustrative -->
```sh
go install github.com/TeachMeHIPAA/teachmehipaa-cli/cmd/tmh@main
```

Or build it from a checkout, which puts the binary in `bin/tmh`:

<!-- doctest: illustrative -->
```sh
git clone https://github.com/TeachMeHIPAA/teachmehipaa-cli.git
cd teachmehipaa-cli
make build
```

A build from source reports its version as `dev`. After the first release,
`tmh` will also install with Homebrew, Scoop, deb and rpm packages, Docker
and release archives; [Getting started](docs/guides/getting-started.md#install)
lists them.

## Quickstart

Create an API client in the Sandbox Environment of the TeachMeHIPAA console
to get a client id (`cli_…`) and secret (`tmh_cs_…`), then put them in the
environment. Sandbox Learners are never emailed, so you can experiment
freely.

<!-- doctest: readme-quickstart -->
```console
$ export TMH_CLIENT_ID=cli_01JD7Z3M1V4Q8T2K5N9R6W0XYZ
$ export TMH_CLIENT_SECRET=tmh_cs_...
$ tmh whoami
Organization  org_acme
Scopes        learners:read, learners:write
Environment   Sandbox
$ tmh learners list --status in_progress --limit 1
{
  "items": [
    {
      "learnerId": "sub_01JD80AB2C3D4E5F6G7H8J9K0M",
      "email": "ana@example.com",
      "name": "Ana Example",
      "externalId": "hr-123",
      "status": "in_progress",
      "course": {
        "courseId": "hipaa-2026",
        "title": "HIPAA Training 2026"
      },
      "invitedAt": "2026-11-01T09:00:00.000Z",
      "startedAt": "2026-11-02T10:30:00.000Z",
      "completedAt": null,
      "dueAt": "2026-12-01T00:00:00.000Z",
      "certificate": null
    }
  ],
  "nextCursor": "eyJvZmZzZXQiOjF9"
}
```

`tmh login` stores credentials in your OS keyring instead, so you don't need
the variables. Every API operation is a `tmh <resource> <method>` command;
`tmh <resource> --help` lists them. Output is JSON on stdout, so `tmh … | jq`
works, and each failure class has its own [exit code](#exit-codes).

## Documentation

| Guide | What it covers |
|---|---|
| [Getting started](docs/guides/getting-started.md) | Install, credentials, first calls, inviting a Learner |
| [Authentication and profiles](docs/guides/authentication-and-profiles.md) | `tmh login`, the `sandbox` and `live` profiles, CI credentials, Scopes, public lookups |
| [Commands](docs/guides/commands.md) | Arguments, flags, `-d` and `--data`, ETags and `--if-match`, idempotency, retries, raw requests |
| [Output and scripting](docs/guides/output-and-scripting.md) | JSON output, `jq`, pagination with `--all` and `--ndjson`, `--verbose` |
| [Webhooks and `events tail`](docs/guides/webhooks-and-events-tail.md) | The Event log, forwarding Events to a local handler, the local signature, Test deliveries |
| [Operations and files](docs/guides/operations-and-files.md) | `--wait`, CSV exports, Certificate PDFs, CSV imports |
| [Exit codes and errors](docs/guides/exit-codes.md) | Every exit code, how errors print, request ids |
| [Configuration and privacy](docs/guides/configuration-and-privacy.md) | Environment variables, config and state files, the version check, shell completion |

- [Command reference](docs/reference/commands/tmh.md): every command and
  flag, generated from `tmh` itself.
- [Resource reference](docs/reference/resources.md): every API operation
  with its command, HTTP route, Scopes, retries and idempotency.
- [Examples](examples/README.md): shell scripts for a roster sync, a
  Certificate export and a webhook development loop.
- [All documentation](docs/README.md), including the design records.

## Exit codes

| Exit code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Usage error or local failure: a bad flag, argument or input file, or anything that went wrong on this machine rather than at the API, such as an unreadable config file. |
| `2` | Auth failed: not logged in, `401`, `403`, or the token endpoint refused the credentials. |
| `3` | Not found: `404`, or something `tmh` looked for isn't there, such as an Event for `events resend`. |
| `4` | Conflict or precondition failed: `409`, `412` or `428`. |
| `5` | Invalid request: any other `4xx`, such as `400` or `422`. |
| `6` | Rate limited: `429`, after retries. |
| `7` | Server or connection error: `5xx`, no response, a response that isn't the API's, or under `--wait` a timed-out Operation or a failed one whose problem has no status. |
| `130` | Interrupted with Ctrl-C. |

[Exit codes and errors](docs/guides/exit-codes.md) shows how errors print and
how to branch on them in a script.

## Support and security

<!-- TODO(Nick): confirm the support address before launch. -->

For help, contact TeachMeHIPAA support and quote the request id `tmh` prints
with every API error. To report a vulnerability, see
[SECURITY.md](SECURITY.md); please don't open a public issue.

`tmh` sends no telemetry and never logs request or response bodies; see
[Configuration and privacy](docs/guides/configuration-and-privacy.md#privacy).
See the [changelog](CHANGELOG.md) for what changed, and
[CONTRIBUTING.md](CONTRIBUTING.md) to work on `tmh`. It is released under the
[MIT License](LICENSE).
