# Changelog

All notable changes to `tmh` are documented here. It follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and
[semantic versioning](https://semver.org/). GitHub release notes are
generated from the conventional commits since the previous tag.

## [Unreleased]

The first release, to ship as 1.0.0 when the Developer API launches.
Generated from the Developer API document release `trimmed-2026-11-15`.

### Added

- A `tmh <resource> <method>` command for each of the 43 Developer API
  operations besides the token endpoint (45 commands), generated from the
  shared SDK IR, with path ids as arguments, body fields as flags,
  `-d key=value`, `--data @file.json`, and the API's own descriptions and
  Scopes in `--help`.
- `tmh login`, `logout` and `whoami`, with `sandbox` and `live` profiles,
  client secrets in the OS keyring (or a 0600 config file without one), and
  `TMH_CLIENT_ID` / `TMH_CLIENT_SECRET` for CI.
- Access tokens fetched and cached for the life of a command, and the
  Organization bound through introspection.
- Pretty, colored JSON on a terminal and compact JSON when piped; `--all` and
  `--ndjson` on list commands.
- Generated Idempotency-Keys for writes, `--idempotency-key`, `--if-match`,
  and retries of transient failures like the SDKs'.
- Problem details rendered on stderr with distinct exit codes per failure
  class (0–7, and 130 when interrupted); `--verbose` and `--show-headers`.
- `--wait` on commands that return an Operation, `-o` for exports and
  Certificate PDFs, and `tmh enrollments import --file roster.csv
  [--preview]`.
- `tmh events tail`, which resumes where it stopped, filters with `--types`,
  and with `--forward-to` POSTs each Event to a local endpoint signed like a
  Delivery with a per-profile local Signing secret; `tmh events resend` and
  `tmh webhooks trigger-test`.
- Raw `tmh get|post|patch|put|delete <path>` requests, unauthenticated for
  the API's public paths or with `--no-auth`, `tmh open`, shell
  completion for bash, zsh, fish and PowerShell, `tmh version`, and a daily
  new-version notice that `TMH_NO_UPDATE_CHECK` turns off. No telemetry.
- Release builds for macOS, Linux and Windows on amd64 and arm64: archives,
  a Homebrew cask, a Scoop manifest, deb and rpm packages, and a
  `ghcr.io/teachmehipaa/tmh` image, with SHA-256 checksums signed by
  Sigstore.

### Documentation

- Guides in `docs/guides/`: getting started, authentication and profiles,
  commands, output and scripting, webhooks and `events tail`, Operations and
  files, exit codes and errors, and configuration and privacy. Every
  `console` session in them and in the README runs against a fake API in
  the test suite and must print what the docs show; every other `tmh`
  command in the docs is checked against the real command tree.
- A command reference in `docs/reference/commands/`, generated from `tmh`'s
  command tree with `cobra/doc` (`make docs`), and
  `docs/reference/resources.md`, generated from the SDK IR: every API
  operation with its command, HTTP route, Scopes, retries, idempotency and
  conditional behavior. Both are checked for staleness by `make lint` and the
  tests.
- Example scripts in `examples/`: a quickstart, a roster sync by External ID,
  a Certificate export with `--wait -o`, and an `events tail --forward-to`
  development loop, checked with ShellCheck in CI.
- A documentation site built with GitHub Pages' Jekyll on every pull request
  and published from `main`, and a check that every relative link and anchor
  in the docs resolves.
- A shorter README with a pre-release status callout that links to the
  guides, and `docs/README.md`, an index of all documentation.
- `docs/reference/code-samples.json`, one `tmh` shell sample per Developer
  API operationId for the API reference's `cli` tab, generated from the SDK
  IR and checked against the command tree; the guides reorganized as topic
  pages in `docs/topics/` that open with a synopsis, plus a recipes page of
  shell workflows.
