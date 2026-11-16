# tmh

`tmh` is the command-line client for the TeachMeHIPAA Developer API. One
static binary covers Learners, Enrollments, Certificates, exports and webhook
Events, from an interactive shell or a CI job.

[![CI](https://github.com/TeachMeHIPAA/teachmehipaa-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/TeachMeHIPAA/teachmehipaa-cli/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

> [!WARNING]
> **Pre-release.** The Developer API is not generally available, and `tmh`
> has no tagged release. Homebrew, Scoop, deb, rpm and the Docker image
> arrive with 1.0. Until then, build from source with the commands below.
> Commands, flags and output can change before 1.0.

## Install

<!-- doctest: illustrative -->
```sh
go install github.com/TeachMeHIPAA/teachmehipaa-cli/cmd/tmh@main
```

Needs Go 1.24 or newer. The binary lands in `$(go env GOPATH)/bin`. To build
from a checkout instead:

<!-- doctest: illustrative -->
```sh
git clone https://github.com/TeachMeHIPAA/teachmehipaa-cli.git
cd teachmehipaa-cli && make build    # writes bin/tmh
```

A source build reports its version as `dev`. [Install](docs/topics/install.md)
lists the release channels, checksum verification, Docker and shell
completion.

## Log in

<!-- doctest: illustrative -->
```console
$ tmh login
Client ID: cli_01JD7Z3M1V4Q8T2K5N9R6W0XYZ
Client secret:
Logged in to Sandbox as Organization org_acme.
```

The client id (`cli_…`) and secret (`tmh_cs_…`) belong to an API client
that an Organization admin creates in the console (`tmh open console`).
`tmh login` stores the secret in the OS keyring. Every command runs against
the `sandbox` profile unless you pass `--live`, and Sandbox Learners never
receive email.

## First command

<!-- doctest: readme-quickstart -->
```console
$ tmh whoami
Organization  org_acme
Scopes        learners:read, learners:write
Environment   Sandbox
$ tmh compliance retrieve
{
  "asOf": "2026-11-15T12:00:00.000Z",
  "courseId": "hipaa-2026",
  "totals": {
    "learners": 42,
    "compliant": 30,
    "expiringSoon": 3,
    "expired": 1,
    "inProgress": 5,
    "notStarted": 2,
    "inviteBounced": 1
  },
  "compliantRate": 0.714
}
```

`tmh` fetched an Access token, asked the API which Organization the
credentials belong to, and put that Organization in the request path. You
never pass an Organization id or a token.

## Usage

<!-- doctest: synopsis -->
```text
tmh learners list [--status <status>] [--all [--ndjson]]
tmh learners retrieve <learnerId>
tmh learners create --email <email> [--name <name>] [-d <key>=<value>]
tmh certificates export --format csv -o <file>
tmh events tail [--forward-to <url>]
tmh get <path> [--query <key>=<value>]
```

Every API operation is `tmh <resource> <method>`. Path ids are positional
arguments; query parameters and body fields are flags. `tmh <resource>
--help` lists the methods. Responses print as JSON on stdout; errors, notices
and `--verbose` lines go to stderr.

| Global flag | Effect |
|---|---|
| `--live` | Use the `live` profile, the same as `--profile live`. Without it, `tmh` uses `sandbox`. |
| `--verbose` | One line per HTTP attempt on stderr: method, path, status, latency, request id. |
| `--show-headers` | Each response's headers on stderr, with credentials redacted. |
| `--max-retries <n>` | Retries of a transient failure on a request that is safe to replay. Default 2. |
| `--idempotency-key <key>` | Your own `Idempotency-Key` for a write, in place of the generated one. |

[Recipes](docs/topics/recipes.md) has shell workflows built from these: jq
pipelines, CSV exports, tailing Events, branching on exit codes.

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

[Exit codes](docs/topics/exit-codes.md) shows the error output for each.

## Documentation

| Topic | Read it for |
|---|---|
| [install](docs/topics/install.md) | `go install`, release channels, checksums, Docker, shell completion |
| [login](docs/topics/login.md) | `tmh login`, the `sandbox` and `live` profiles, CI credentials, Scopes |
| [requests](docs/topics/requests.md) | Arguments, flags, `-d` and `--data`, ETags, idempotency, retries, raw requests |
| [output](docs/topics/output.md) | JSON on stdout, pages, `--all`, `--ndjson`, `--verbose` |
| [operations](docs/topics/operations.md) | `--wait`, CSV exports, Certificate PDFs, CSV imports, batch changes |
| [events](docs/topics/events.md) | The event log, `events tail --forward-to`, local signatures, Test deliveries |
| [exit-codes](docs/topics/exit-codes.md) | Each exit code and the error it prints |
| [environment](docs/topics/environment.md) | Environment variables, config and state files, privacy, the version check |
| [recipes](docs/topics/recipes.md) | Shell workflows: jq, CSV files, `while read` loops, exit codes |

- [Resource reference](docs/reference/resources.md): each API operation's
  command, route, Scopes and retry behavior, with an example.
- [Command reference](docs/reference/commands/tmh.md): every command and
  flag, generated from the command tree.
- [Examples](examples/README.md): scripts for a roster sync, a Certificate
  export and a webhook development loop.
- [Documentation index](docs/README.md), including the design records.

## Support and security

<!-- TODO(Nick): confirm the support address before launch. -->

Quote the request id from the error output when you contact TeachMeHIPAA
support. Report vulnerabilities as [SECURITY.md](SECURITY.md) describes,
never in a public issue. `tmh` sends no telemetry and logs no request or
response bodies ([environment](docs/topics/environment.md#privacy)).

[Changelog](CHANGELOG.md) · [Contributing](CONTRIBUTING.md) ·
[MIT License](LICENSE)
