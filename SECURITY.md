# Security policy

<!-- TODO(Nick): confirm the reporting address and response times before launch. -->

## Reporting a vulnerability

Please report security issues privately to **security@teachmehipaa.com**, not
in a public GitHub issue. Include the `tmh version` output, what you found
and, if you can, steps to reproduce it. Don't include real Learner data,
Access tokens, client secrets or Signing secrets in the report.

We will acknowledge your report, keep you informed while we fix it, and
credit you in the release notes if you would like.

## Supported versions

Security fixes are released for the latest major version.

## How tmh handles secrets and Learner data

- It sends no telemetry. Besides the API and your `--forward-to` URL, its
  only request is a daily version check to the GitHub releases API, which
  `TMH_NO_UPDATE_CHECK` turns off (ADR-0007).
- Client secrets and local Signing secrets go into the OS keyring. Where no
  keyring works, they go into `~/.config/tmh/config.toml`, which is written
  atomically with mode 0600.
- It never prints request or response bodies or credentials in `--verbose`,
  `--show-headers` or error output, and it redacts `Authorization` and
  cookie headers.
- Access tokens live only in memory, for one command.
- Forwarded Events go only to the URL you pass, through a separate HTTP
  client, and redirects are not followed.

## Verifying a release

Each release's `checksums.txt` is signed with keyless Sigstore by the
release workflow. The README's "Verify a download" section shows the
`cosign verify-blob` command.
