# Configuration and privacy

`tmh` needs no configuration beyond your credentials. This guide lists what
you can change, the files `tmh` writes, everything it connects to, and how to
set up shell completion.

## Environment variables

| Variable | Effect |
|---|---|
| `TMH_CLIENT_ID`, `TMH_CLIENT_SECRET` | Use these credentials instead of stored ones, so CI needs no `tmh login`. Set both or neither. The profile still picks the Environment. |
| `TMH_NO_UPDATE_CHECK` | Any non-empty value turns off the [version check](#the-version-check). |
| `CI` | Any non-empty value also turns off the version check. |
| `NO_COLOR` | Print JSON without color, even on a terminal. |
| `XDG_CONFIG_HOME` | Keep `tmh`'s files in `$XDG_CONFIG_HOME/tmh` instead of `~/.config/tmh`. |

## Global flags

These work on every command:

| Flag | Default | Effect |
|---|---|---|
| `--profile <name>` | `sandbox` | The profile, and so the Environment: `sandbox` or `live`. |
| `--live` | off | The same as `--profile live`. |
| `--max-retries <n>` | `2` | Retries of a transient failure of a request that is safe to replay; `0` turns them off. |
| `--idempotency-key <key>` | generated | The write's `Idempotency-Key`. |
| `--verbose` | off | One line per HTTP attempt on stderr. |
| `--show-headers` | off | Each response's headers on stderr. |

Commands that wait for an Operation also take `--wait-timeout` (5 minutes by
default), and `tmh events tail` takes `--interval` (2 seconds). The
[command reference](../reference/commands/tmh.md) lists every flag with its
default.

## Configuration and state files

`tmh` writes only these files, in `~/.config/tmh` on every OS
(`$XDG_CONFIG_HOME/tmh` when that is set). Both are written atomically and
readable only by you.

| File | Holds |
|---|---|
| `config.toml` | Each profile's client id and Organization id. A client secret or local Signing secret is here only when the OS keyring is unusable. |
| `state.toml` | When the version check last ran, and where `events tail` stopped for each profile and Organization. |

Secrets live in the OS keyring under the service name `tmh`: the macOS
Keychain, the Windows Credential Manager, or the Secret Service on Linux.
`tmh logout` removes a profile's client secret; its local Signing secret
stays, so your webhook handler keeps working.

## Privacy

`tmh` sends no telemetry and never logs request or response bodies. It
connects only to:

1. the TeachMeHIPAA API, for the commands you run;
2. the version check's GitHub releases API, unless you turn it off;
3. what you ask for yourself: your `--forward-to` URL, which gets nothing
   but the forwarded Event.

`--verbose` and `--show-headers` never print a body or a credential:
`Authorization` and cookie headers are redacted. Errors print only what the
API's response said.

## The version check

The version check asks
`https://api.github.com/repos/TeachMeHIPAA/teachmehipaa-cli/releases/latest`
for the latest release at most once a day, while your command runs, and
prints one line on stderr if a newer version exists. It never goes through
the connection `tmh` uses for the API, sends nothing but a `User-Agent` of
`teachmehipaa-cli/<version> go/<version> <os>/<arch>`, waits at most one
second, and can never fail your command. It is skipped when stderr isn't a
terminal, when `CI` is set, for completion scripts, and for builds without a
release version, such as a build from source. Set `TMH_NO_UPDATE_CHECK=1` to
turn it off.

## Shell completion

`tmh completion <shell>` prints a completion script for bash, zsh, fish or
PowerShell. Once `tmh` is released, Homebrew and the deb and rpm packages
install completions for bash, zsh and fish for you. Otherwise, load it from
your shell's startup file:

<!-- doctest: illustrative -->
```sh
source <(tmh completion bash)                          # ~/.bashrc
source <(tmh completion zsh)                           # ~/.zshrc
tmh completion fish | source                           # ~/.config/fish/config.fish
```

In PowerShell, add `tmh completion powershell | Out-String | Invoke-Expression`
to your `$PROFILE`. `tmh completion <shell> --help` shows how to install the
script permanently. Completion offers resources, methods, flags and enum
values.
