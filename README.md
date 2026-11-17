# tmh

`tmh` is the official command-line client for the TeachMeHIPAA Developer API.

[![CI](https://github.com/TeachMeHIPAA/teachmehipaa-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/TeachMeHIPAA/teachmehipaa-cli/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

## Documentation

**[docs.teachmehipaa.com/api](https://docs.teachmehipaa.com/api?lang=cli)**
has the API reference with a `tmh` sample for every operation, plus guides
for authentication, pagination, errors and webhooks. `tmh open docs` opens
it, and `tmh <command> --help` documents every command and flag.

## Install

> [!WARNING]
> **Pre-release.** The Developer API is not generally available, and `tmh`
> has no tagged release. Homebrew, Scoop, deb, rpm and the Docker image
> arrive with 1.0. Until then, build from source. Commands, flags and output
> can change before 1.0.

<!-- doctest: illustrative -->
```sh
go install github.com/TeachMeHIPAA/teachmehipaa-cli/cmd/tmh@main
```

The binary lands in `$(go env GOPATH)/bin`. From a checkout,
`make build` writes `bin/tmh`.

## Requirements

Go 1.24 or newer to build. An API client from the TeachMeHIPAA console:
a client id (`cli_…`) and secret (`tmh_cs_…`).

## Usage

<!-- doctest: illustrative -->
```console
$ tmh login
Client ID: cli_01JD7Z3M1V4Q8T2K5N9R6W0XYZ
Client secret:
Logged in to Sandbox as Organization org_acme.
```

<!-- doctest: readme-quickstart -->
```console
$ tmh whoami
Organization  org_acme
Scopes        learners:read, learners:write
Environment   Sandbox
$ tmh compliance retrieve > compliance.json
```

Every API operation is `tmh <resource> <method>`, and responses print as
JSON on stdout. Commands run against the Sandbox
(`https://sandbox.api.tme.so/v1`) unless you pass `--live`.

## Exit codes

| Exit code | Meaning |
|---|---|
| `0` | Success. |
| `1` | Usage error or local failure. |
| `2` | Auth failed: not logged in, `401`, `403`. |
| `3` | Not found: `404`. |
| `4` | Conflict or precondition failed: `409`, `412` or `428`. |
| `5` | Invalid request: any other `4xx`. |
| `6` | Rate limited: `429`, after retries. |
| `7` | Server or connection error: `5xx` or no response. |
| `130` | Interrupted with Ctrl-C. |

## Support

<!-- TODO(Nick): confirm the support address before launch. -->

Quote the request id from the error output when you contact TeachMeHIPAA
support. `tmh` sends no telemetry and logs no request or response bodies.
Changes are in the [changelog](CHANGELOG.md).

## Security

Report vulnerabilities as [SECURITY.md](SECURITY.md) describes, never in a
public issue.

## Contributing

[CONTRIBUTING.md](CONTRIBUTING.md) covers building, testing and the
generators. The [command reference](docs/reference/commands/tmh.md) is
generated from the command tree.

## License

[MIT](LICENSE)
