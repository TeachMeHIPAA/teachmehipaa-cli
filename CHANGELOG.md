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
  operations besides the token endpoint (45 commands), generated from the shared SDK IR, with path ids as arguments,
  body fields as flags, `-d key=value`, `--data @file.json`, and the API's
  own descriptions and Scopes in `--help`.
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
