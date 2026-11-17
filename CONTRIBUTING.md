# Contributing to tmh

## Setup

You need Go 1.24 or newer. GoReleaser 2.x and Docker are needed only to build
release artifacts.

```sh
make build        # bin/tmh
make test         # go test -race ./...
make lint         # go vet, the generated-file checks, ShellCheck, gofmt and staticcheck
make check        # lint and test
make docs         # regenerate the command reference after changing a command
```

CI runs the tests on Linux and macOS with Go 1.24 and the latest Go, `make
lint`, ShellCheck, and `goreleaser check`. Every change must pass all of
them. `make lint` skips
ShellCheck when it isn't installed (`brew install shellcheck`).

## Layout

| Path | What it is |
|---|---|
| `cmd/tmh` | `main`: wires the real transport, keyring, clock and terminal into the command tree. |
| `cmd/gen`, `internal/gen` | The generator for the resource commands and `docs/reference/code-samples.json`. |
| `cmd/docgen`, `internal/docgen` | The generator for the command reference in `docs/reference/commands/`. |
| `internal/doctest` | Reads the docs for the documentation tests: code blocks, links, anchors and shell commands. |
| `internal/commands` | The root command and the hand-written commands. |
| `internal/commands/generated` | Generated resource commands. Never edit them by hand. |
| `internal/api` | The HTTP client: tokens, retries, idempotency, problem details. |
| `internal/events` | The `events tail` forwarder: canonical JSON and the local signature. |
| `internal/config` | Profiles, `config.toml`, `state.toml` and the keyring store. |
| `internal/testkit` | The fake transport and harness every command test runs through. |
| `openapi/` | The vendored Developer API document and SDK IR. See its README. |
| `docs/reference/` | The command reference and code samples; see [Documentation](#documentation). |
| `examples/` | Shell scripts using `tmh` against the Sandbox. |
| `docs/adr/` | Architecture decisions. Read them before changing behavior they cover. |

Use the terms in `GLOSSARY.md` in names, messages and docs.

## The generator

The resource commands come from `openapi/sdk-ir.json`, the IR the TeachMeHIPAA
monorepo builds from the Developer API document (ADR-0001). To pick up an API
change:

1. Rebuild the IR in the monorepo and copy the three files into `openapi/`, as
   `openapi/README.md` describes, then update `OPENAPI_VERSION`.
2. Run `make generate`. It rewrites the commands, the code samples and the
   command reference.
3. Run `make check`. `internal/commands/generated/coverage_test.go` fails if
   any IR method lacks exactly one command.

`make lint` runs `go run ./cmd/gen -check` and `go run ./cmd/docgen -check`,
which fail when the committed commands or references differ from what the IR
and the command tree generate. To change the commands' shape, change
`internal/gen` and regenerate; never edit generated files.

## Tests

Tests run `tmh <args>` through `internal/testkit`: an injected fake transport,
an in-memory keyring, a fake clock and captured stdout and stderr. Assert on
output, exit code and the requests sent. The `events` package checks
signatures against vectors computed independently from the platform's
canonical string. No test calls the live API or touches the real keyring.

`internal/commands/exit_code_tables_test.go` parses the exit-code table in
the README and fails if it differs from the codes `tmh` returns, so update
them together.

## Documentation

The API reference and guides live at
<https://docs.teachmehipaa.com/api?lang=cli> (what `tmh open docs` opens),
built from the monorepo. This repository keeps only what feeds it or
documents `tmh` itself:

| Path | What it is | Kept in sync by |
|---|---|---|
| `README.md` | Overview, install, quickstart, exit codes, links | The doc tests |
| `docs/reference/commands/*.md` | Every command and flag, from the command tree with `cobra/doc` | `make docs`; `go run ./cmd/docgen -check` and `internal/docgen`'s test |
| `docs/reference/code-samples.json` | One shell sample per IR operationId, synced into the hosted API docs as its `cli` samples | `make generate`; `go run ./cmd/gen -check`, `internal/gen`'s test and `code_samples_test.go` |
| `docs/README.md`, `examples/README.md` | Indexes | Hand-written; links checked |
| `examples/*.sh` | Scripts against the Sandbox | ShellCheck; the doc tests check their `tmh` commands |

Write for partner developers, in the terms of `GLOSSARY.md`. The doc tests
are plain `go test`:

- **Sessions.** A `console` block preceded by `<!-- doctest: <name> -->` is
  run by `internal/commands/docs_test.go` against the fake transport: each
  `$ ` line runs, and what it prints, stderr and stdout interleaved as on a
  terminal, must equal the lines after it. The fake API behind each session
  is in `docs_sessions_test.go`, keyed by the name. Sessions support `tmh`,
  `export`, `echo $?`, `cat <file>`, `> file` and `printf %s "$VAR" |`. A
  command that fails must be followed by `echo $?`, so a failure never reads
  as success.
- **Illustrative examples.** Every other `console`, `sh` or `bash` block must
  be marked `<!-- doctest: illustrative -->`, for examples that can't run
  deterministically, such as `tmh login` prompting or anything piped to
  `jq`. Their `tmh` commands, and those in `examples/*.sh`, are still parsed
  against the real command tree, so a renamed command, flag or argument
  fails the test.
- **jq programs.** Every `jq` program in the illustrative blocks, the example
  scripts and the code samples must compile with the `jq` on `PATH` (the
  test skips without one).
- **Code samples.** `code_samples_test.go` holds `code-samples.json` to the
  IR: one sample per operationId, the release in `OPENAPI_VERSION`, and each
  sample calling its operation's command once, with real flags and every
  required body field set by a flag, `-d` or `--data`. The sample values
  and the optional flag each sample shows live in `internal/gen/samples.go`.
- **Links.** `internal/doctest` checks that every relative link and `#anchor`
  in every Markdown file resolves, with GitHub's heading anchors.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/) (`feat:`,
`fix:`, `docs:`, `chore:`…). GitHub release notes are generated from them:
`feat` and `fix` commits get their own sections, `!` marks a breaking change,
and `docs`, `test`, `chore` and `ci` commits are left out. Add user-facing
changes to `CHANGELOG.md` under `[Unreleased]` too.

## Releasing

Releases are built by GoReleaser from `.goreleaser.yaml` and run only from the
**Release** workflow, which is `workflow_dispatch` only. By default it builds a
snapshot of every artifact and uploads it to the workflow run, publishing
nothing. Run the same build locally with:

```sh
make snapshot     # goreleaser release --snapshot --clean
./dist/tmh_darwin_arm64*/tmh version
```

Without Docker, skip the image with
`goreleaser release --snapshot --clean --skip=docker`.

### One-time setup (Nick, before the first release)

Nothing has been published. These steps must happen first:

1. **Make the repo public** (done). Homebrew, Scoop, `go install` and the
   deb and rpm links all download release assets anonymously.
2. **Create the tap and bucket repos**, each with a README and a `main`
   branch:
   - `TeachMeHIPAA/homebrew-tap`: the cask is written to `Casks/tmh.rb`.
   - `TeachMeHIPAA/scoop-bucket`: the manifest is written to
     `bucket/tmh.json`.
3. **Create a token that can push to both**: a fine-grained personal access
   token or GitHub App token with `Contents: read and write` on those two
   repos only.
4. **Create a `release` environment** in this repo's settings, with you as a
   required reviewer and deployments limited to tags matching `v*`. Add the
   token there as the secret `TAP_GITHUB_TOKEN`. (A run creates the
   environment without protection if it doesn't exist, so create it first.)
5. After the first publish, open the `tmh` package on GitHub (Packages), link
   it to this repo, and make it public; new GHCR packages start private.

Without `TAP_GITHUB_TOKEN`, GoReleaser skips the tap and bucket, and the
publish job refuses to run.

### Each release

1. Move the `[Unreleased]` notes in `CHANGELOG.md` under the new version and
   date, and commit that on `main`.
2. Tag the commit and push the tag:

   ```sh
   git tag -a v1.0.0 -m "tmh 1.0.0"
   git push origin v1.0.0
   ```

3. **Dry run.** Actions → Release → Run workflow, choose the tag as the ref,
   and leave **publish** unchecked. Download the `tmh-snapshot` artifact and
   check an archive, the cask and the Scoop manifest.
4. **Publish.** Run the workflow again on the tag with **publish** checked,
   then approve the `release` environment. The job fails unless the ref is a
   `v*` tag. It publishes the GitHub release with archives, packages and
   `checksums.txt`; pushes `ghcr.io/teachmehipaa/tmh:<version>` and `:latest`
   for amd64 and arm64; updates the tap and bucket; and signs
   `checksums.txt` with keyless Sigstore. A tag with a prerelease suffix,
   such as `v1.1.0-rc.1`, is marked a prerelease, skips the tap and bucket,
   and doesn't move `latest`.
5. Check the release: `brew install --cask teachmehipaa/tap/tmh`,
   `docker run --rm ghcr.io/teachmehipaa/tmh version`, and the `cosign
   verify-blob` command from the README.

### Not set up yet

- **macOS signing and notarization.** The binary isn't notarized, so the
  cask removes the quarantine attribute after install. Notarizing needs an
  Apple Developer ID certificate and an App Store Connect API key
  (GoReleaser's `notarize.macos`).
- **Windows Authenticode signing**, **winget**, an install script and an
  npm wrapper, which the Stripe CLI research lists as later additions.
