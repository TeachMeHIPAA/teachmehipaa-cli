# Environment, files and privacy

<!-- doctest: synopsis -->
```text
tmh whoami --profile live
tmh learners list --max-retries 0
tmh learners create --email <email> --idempotency-key <key>
```

Credentials are the only configuration `tmh` needs. The rest of this page
covers what you can change and what `tmh` writes and connects to.

## Environment variables

| Variable | Effect |
|---|---|
| `TMH_CLIENT_ID`, `TMH_CLIENT_SECRET` | Credentials that override stored ones, so CI needs no `tmh login`. Set both or neither. The profile still picks the Environment. |
| `TMH_NO_UPDATE_CHECK` | Any non-empty value turns off the [version check](#the-version-check). |
| `CI` | Any non-empty value turns off the version check too. |
| `NO_COLOR` | JSON prints without color, even on a terminal. |
| `XDG_CONFIG_HOME` | `tmh` keeps its files in `$XDG_CONFIG_HOME/tmh` in place of `~/.config/tmh`. |

## Global flags

Every command takes these:

| Flag | Default | Effect |
|---|---|---|
| `--profile <name>` | `sandbox` | The profile, which picks the Environment: `sandbox` or `live`. |
| `--live` | off | Same as `--profile live`. |
| `--max-retries <n>` | `2` | Retries of a transient failure on a request that is safe to replay; `0` turns them off. |
| `--idempotency-key <key>` | generated | The write's `Idempotency-Key`. |
| `--verbose` | off | One line per HTTP attempt on stderr. |
| `--show-headers` | off | Each response's headers on stderr. |

Commands that return an Operation add `--wait-timeout` (default 5m), and
`tmh events tail` adds `--interval` (default 2s). The
[command reference](../reference/commands/tmh.md) lists every flag and its
default.

## Configuration and state files

`tmh` writes two files, both in `~/.config/tmh` on every OS
(`$XDG_CONFIG_HOME/tmh` when set). Each write is atomic and leaves the file
readable by you alone.

| File | Holds |
|---|---|
| `config.toml` | Each profile's client id and Organization id. A client secret or local Signing secret lands here only when the OS keyring is unusable. |
| `state.toml` | When the version check last ran, and where `events tail` stopped for each profile and Organization. |

Secrets live in the OS keyring under the service name `tmh`: the macOS
Keychain, the Windows Credential Manager or the Secret Service on Linux.
`tmh logout` deletes a profile's client secret and keeps its local Signing
secret, so your webhook handler's configuration stays valid.

## Privacy

`tmh` sends no telemetry and logs no request or response bodies. It
connects to:

1. the TeachMeHIPAA API, for the commands you run;
2. the GitHub releases API, for the version check, unless you turn it off;
3. your `--forward-to` URL, which receives the forwarded Event and nothing
   else.

`--verbose` and `--show-headers` never print a body or a credential:
`Authorization` and cookie headers print as `[redacted]`. Errors print only
what the API's response said.

## The version check

At most once a day, while your command runs, `tmh` asks
`https://api.github.com/repos/TeachMeHIPAA/teachmehipaa-cli/releases/latest`
for the newest release and prints one line on stderr if it is newer than
yours. The check uses its own HTTP connection, separate from the API's. It
sends a `User-Agent` of
`teachmehipaa-cli/<version> go/<version> <os>/<arch>` and nothing else,
waits at most one second, and never fails your command.

It is skipped when stderr isn't a terminal, when `CI` is set, for completion
scripts, and for builds without a release version, such as a source build.
`TMH_NO_UPDATE_CHECK=1` turns it off.

Shell completion is set up in [install](install.md#shell-completion).
