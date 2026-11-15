# tmh: the TeachMeHIPAA CLI

`tmh` works with the TeachMeHIPAA Developer API from your terminal. Explore the
API, script admin tasks, and test your webhook handler against real Events
without a public URL. It is a single static binary with no runtime
dependencies.

> [!NOTE]
> The Developer API has not launched yet, and `tmh` has no published release.
> The install commands below work only after the first release. Until then,
> build it from a checkout (see [CONTRIBUTING.md](CONTRIBUTING.md)).

## Contents

- [Install](#install)
- [Log in](#log-in)
- [Commands](#commands)
- [Output](#output)
- [Operations and `--wait`](#operations-and---wait)
- [Downloads and imports](#downloads-and-imports)
- [Testing webhooks with `events tail`](#testing-webhooks-with-events-tail)
- [Raw requests](#raw-requests)
- [Exit codes](#exit-codes)
- [Debugging](#debugging)
- [Configuration and state files](#configuration-and-state-files)
- [Privacy](#privacy)
- [Shell completion](#shell-completion)

## Install

These channels work after the first release.

| Platform | Command |
|---|---|
| macOS, Linux (Homebrew) | `brew install --cask teachmehipaa/tap/tmh` |
| Windows (Scoop) | `scoop bucket add teachmehipaa https://github.com/TeachMeHIPAA/scoop-bucket`<br>`scoop install teachmehipaa/tmh` |
| Debian, Ubuntu | download `tmh_<version>_<arch>.deb` from [Releases](https://github.com/TeachMeHIPAA/teachmehipaa-cli/releases), then `sudo dpkg -i tmh_<version>_amd64.deb` |
| Fedora, RHEL | download `tmh-<version>-1.<arch>.rpm` from Releases, then `sudo rpm -i tmh-<version>-1.x86_64.rpm` |
| Docker | `docker run --rm ghcr.io/teachmehipaa/tmh version` |
| Go 1.24+ | `go install github.com/TeachMeHIPAA/teachmehipaa-cli/cmd/tmh@latest` |
| Anything else | download the archive for your OS and CPU from Releases and put `tmh` on your `PATH` |

Release archives are `.tar.gz` (`.zip` on Windows) for macOS, Linux and Windows
on amd64 and arm64. Each holds `tmh`, this README, the license, the changelog
and completion scripts. The deb and rpm packages install the completions for
bash, zsh and fish too.

A `go install` build reports its version as `dev`. Release builds report the
version, commit and build date:

```console
$ tmh version
tmh version 1.0.0 (commit 39276f1, built 2026-11-15T19:27:47Z)
```

### Verify a download

`checksums.txt` lists the SHA-256 of every archive and package. Each release
also has `checksums.txt.sigstore.json`, a keyless Sigstore signature made by
the release workflow:

```sh
cosign verify-blob checksums.txt \
  --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/TeachMeHIPAA/teachmehipaa-cli/\.github/workflows/release\.yml@refs/tags/v' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
sha256sum --check --ignore-missing checksums.txt
```

### Docker

The image runs as a non-root user and has no OS keyring, so pass credentials
as environment variables:

```sh
docker run --rm -e TMH_CLIENT_ID -e TMH_CLIENT_SECRET \
  ghcr.io/teachmehipaa/tmh learners list --limit 5
```

To keep the [local Signing secret](#testing-webhooks-with-events-tail) and the
`events tail` resume point between runs, mount a volume at
`/home/nonroot/.config/tmh`.

## Log in

Create an API client in the TeachMeHIPAA console (`tmh open console`). You get
a client id (`cli_…`) and a client secret. Then:

```console
$ tmh login
Client ID: cli_01JD7Z…
Client secret:
Logged in to Sandbox as Organization org_acme.
```

`tmh login` checks the credentials with TeachMeHIPAA before storing them. The
secret goes into your OS keyring (Keychain, Windows Credential Manager, or the
Secret Service on Linux). Where no keyring works, such as most CI containers,
it goes into `config.toml` instead, which is readable only by you, and `tmh`
says so.

Without a terminal, pass the id as a flag and pipe the secret on stdin:

```sh
printf %s "$SECRET" | tmh login --client-id cli_01JD7Z…
```

### Profiles

Credentials belong to one Environment, so `tmh` has one profile for each:

| Profile | Environment | API base URL |
|---|---|---|
| `sandbox` (the default) | Sandbox | `https://api-teachmehipaa-dev.web.app/v1` |
| `live` | Live | `https://api.tme.so/v1` |

Every command uses `sandbox` unless you pass `--live` (or `--profile live`), so
you never reach Live by accident:

```sh
tmh login --live
tmh learners list --live
```

`tmh whoami` shows the Organization, Scopes and Environment of the selected
profile's credentials. `tmh logout` (or `tmh logout --live`) removes them.

### Environment variables

| Variable | Effect |
|---|---|
| `TMH_CLIENT_ID`, `TMH_CLIENT_SECRET` | Use these credentials instead of stored ones, so CI needs no `tmh login`. Set both or neither. The profile still picks the Environment. |
| `TMH_NO_UPDATE_CHECK` | Any non-empty value turns off the [version check](#privacy). |
| `CI` | Any non-empty value also turns off the version check. |
| `NO_COLOR` | Print JSON without color, even on a terminal. |
| `XDG_CONFIG_HOME` | Keep `tmh`'s files in `$XDG_CONFIG_HOME/tmh` instead of `~/.config/tmh`. |

`tmh` fetches Access tokens itself and keeps them in memory for the life of
one command. You never handle a token.

## Commands

Every API operation has a command named `tmh <resource> <method>`, generated
from the same description of the API as the SDKs:

```sh
tmh learners list --status in_progress
tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M
tmh enrollments cancel enr_01JD8C --refund
tmh webhook-subscriptions test whs_01JD8W
```

The resources are `audit-events`, `auth`, `certificate-verifications`,
`certificates`, `compliance`, `courses`, `enrollments`, `events`, `learners`,
`members`, `operations`, `partners`, `payment-method`, `seat-orders`, `seats`
and `webhook-subscriptions` (also spelled `webhooks`). `tmh <resource> --help`
lists the methods, and `tmh <resource> <method> --help` shows the API's own
description of the operation, its Scopes and its flags.

- **Path ids are positional arguments**, in the order the path has them. The
  Organization id is never one: `tmh` fills it in from your credentials.
- **Body fields are flags.** Names are kebab-case, and nested fields are
  dotted: `--course-id`, `--options.course-id`. Enum flags list their values
  in `--help`.
- **`-d key=value`** sets any body field, flags or not. Dotted keys nest
  (`-d options.courseId=hipaa-2026`), `key[]=v` appends to an array, and
  `key:=<json>` sends a JSON value instead of a string (`-d externalId:=null`,
  `-d seats:=5`).
- **`--data @file.json`** sends a whole JSON document as the body; `--data @-`
  reads it from stdin, and `--data '{"email":"ana@example.com"}'` takes it
  inline. Flags override `-d` fields, which override the document.
- **`--if-match <etag>`** on updates makes the write conditional.
- **`--idempotency-key <key>`** sets the write's `Idempotency-Key`. Without
  it, `tmh` generates one for every write that takes one, and reuses it for
  each retry, so a retried write never happens twice.

```sh
tmh learners create --email ana@example.com --name "Ana Example"
tmh learners update sub_01JD80AB2C3D4E5F6G7H8J9K0M --name "Ana B. Example" --if-match '"v7"'
tmh webhook-subscriptions create --data @subscription.json
```

Transient failures (connection errors, `429`, `5xx`, and a `409 conflict`) of
requests that are safe to replay are retried twice with backoff, honoring
`Retry-After`, as the SDKs do. `--max-retries`
changes that, and `--max-retries 0` turns it off.

## Output

Responses print to stdout as JSON: pretty-printed and colored on a terminal,
compact when piped, so `tmh … | jq` just works. Errors, notices and
`--verbose` lines go to stderr, so they never corrupt piped output.

List commands print one page. `--limit` sets the page size and `--cursor`
takes the `nextCursor` of the previous page (on `tmh events list`, `--cursor`
is another name for the event log's `--after`):

```sh
tmh learners list --limit 100 --cursor eyJvZmZzZXQiOjEwMH0
```

`--all` follows every page, keeping the other flags, and prints all the items
as one JSON array. Add `--ndjson` to print one compact item per line as each
page arrives instead, which suits large exports:

```sh
tmh learners list --all | jq length
tmh audit-events list --all --ndjson > audit.ndjson
```

## Operations and `--wait`

Bulk commands such as `tmh learners export`, `tmh enrollments import` and the
batch methods start an Operation and return it at once. `--wait` polls it
until it is done, up to `--wait-timeout` (5 minutes by default), and prints
the finished Operation. If the Operation failed, `tmh` prints it and its
problem, and exits with the code the problem's HTTP `status` maps to, as for
any API error (a `422` exits 5, a `5xx` 7), or 7 when the problem has no
status.

```sh
tmh learners export --format csv --wait
tmh learners export --format csv -o learners.csv
```

On exports, `-o <file>` implies `--wait` and saves the finished file. `-o -`
writes it to stdout.

## Downloads and imports

```sh
tmh certificates download-pdf cert_01JD8F -o certificate.pdf
tmh certificates download-pdf cert_01JD8F -o - | lpr

tmh enrollments import --file roster.csv --preview
tmh enrollments import --file roster.csv --wait
```

`--preview` checks the CSV and reports what would happen without inviting
anyone. `--file -` reads the upload from stdin.

## Testing webhooks with `events tail`

`tmh events tail` watches your Organization's Events as they happen. With
`--forward-to`, it POSTs each one to your local server, signed exactly like a
real TeachMeHIPAA Delivery, so you can build your webhook handler without a
public URL:

```console
$ tmh events tail --forward-to http://localhost:3000/webhooks/tmh
Ready! Your local signing secret is 3q2+7wAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA= (^C to quit)
2026-11-15T12:00:00.000Z  learner.certified  evt_01JD8M  [200] POST http://localhost:3000/webhooks/tmh (12ms)
2026-11-15T12:00:04.000Z  learner.expired  evt_01JD8N  [500] POST http://localhost:3000/webhooks/tmh (8ms)
```

Each line shows your server's response status, so you see your handler fail.
Without `--forward-to`, `tail` only prints the Events.

- `tail` polls the event log every 2 seconds (`--interval`), backing off to
  10 seconds while it is quiet.
- It resumes where it last stopped for the profile and Organization, so a
  restart neither misses nor repeats Events. The first run starts with the
  next new Event. `--from-start` starts from the oldest Event in the log
  (it keeps 30 days), and `--after <eventId>` starts after that Event.
- `--types learner.certified,learner.expired` shows only those Event types.
- `tmh events resend <eventId> --forward-to <url>` forwards one Event from
  the log again, for replaying it while you fix a bug.
- `tmh webhooks trigger-test <subscriptionId>` (the same as
  `tmh webhook-subscriptions test`) asks TeachMeHIPAA to send a real Test
  delivery to a registered Webhook subscription.

### The local signature

A forwarded Event is signed the way TeachMeHIPAA signs a Delivery, with two
differences: the key is your profile's **local Signing secret** instead of a
Webhook subscription's, and `x-tmh-key-id` is `local`.

The request is a `POST` with `content-type: application/json`. Its body is the
Event exactly as a Delivery carries it: JSON with every object's keys sorted.
`tmh` signs this canonical string with HMAC-SHA256, using the secret's UTF-8
bytes as the key:

```text
POST
<path and query of the --forward-to URL>
<timestamp>
<SHA-256 of the body, lowercase hex>
```

The lines are joined by `\n` with no trailing newline. The timestamp is the
send time in ISO 8601 UTC with milliseconds, such as
`2026-11-15T12:00:00.000Z`. For `--forward-to
http://localhost:3000/webhooks/tmh?source=cli`, the second line is
`/webhooks/tmh?source=cli`. The signature covers that path, not the host, so a
handler behind a proxy or in a container verifies as long as it sees the same
path. `tmh` doesn't follow redirects, since the signature wouldn't cover the
new path.

The headers are:

| Header | Value |
|---|---|
| `x-tmh-key-id` | `local` |
| `x-tmh-timestamp` | the timestamp in the canonical string |
| `x-tmh-body-hash` | the body's SHA-256, lowercase hex |
| `x-tmh-signature` | the HMAC-SHA256 of the canonical string, lowercase hex |
| `x-tmh-event-type` | the Event's type, such as `learner.certified` |
| `x-tmh-event-id` | the Event's id |

### Verifying forwarded Events

The local Signing secret is created the first time you need it, printed when
`tail` starts, and stays the same across runs, so configure your handler with
it once. Each profile has its own, and `tmh logout` keeps it. Print it with
`--print-secret`:

```sh
export TMH_WEBHOOK_SECRET="$(tmh events tail --print-secret)"
```

Your handler then verifies forwarded Events with its SDK's `constructEvent`
(`construct_event` in Python and Ruby, `ConstructEvent` in .NET), unchanged
from production. Pass the raw body, the headers, the secret and the request
path with its query string. For example, with the Node.js library and Express:

```ts
app.post("/webhooks/tmh", express.raw({ type: "application/json" }), async (req, res) => {
  const event = await webhooks.constructEvent(
    req.body,
    req.headers,
    process.env.TMH_WEBHOOK_SECRET!,
    req.originalUrl,
  );
  console.log(event.eventType);
  res.sendStatus(200);
});
```

The SDKs reject a signature more than 5 minutes from their clock. `tmh` signs
each Event when it sends it, so even an old Event replayed with `--from-start`
or `resend` verifies.

## Raw requests

`tmh get|post|patch|put|delete <path>` calls any API path with the same
authentication, retries and error handling as the generated commands, for
anything they don't cover yet. The path is relative to the base URL, which
already ends in `/v1`, and a literal `{orgId}` is replaced by your
Organization:

```sh
tmh get /organizations/{orgId}/learners --query limit=5
tmh post /organizations/{orgId}/learners -d email=ana@example.com -d name="Ana Example"
tmh patch /organizations/{orgId}/learners/sub_123 -d externalId:=null --if-match '"v7"'
```

`-d`, `--data`, `--if-match` and `--idempotency-key` work as on the generated
commands, `--query key=value` adds a query parameter, and `--content-type`
overrides the body's type. PATCH bodies default to
`application/merge-patch+json`.

A path the API serves publicly (`/courses`, `/courses/<courseId>` and
`/certificate-verifications/<tinyId>`) is sent without an Access token, so it
works before `tmh login`. `--no-auth` sends any other request without one
too.

A raw GET is retried like any read. A raw POST is retried with the
Idempotency-Key `tmh` generates for it, and a raw PUT only when you pass
`--idempotency-key`. A raw PATCH or DELETE is never retried, key or not,
since it may already have taken effect.

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

An API error prints its title and detail, each field error, and the request
id on stderr:

```text
Error: Validation failed: The request has invalid fields.
  learner.email: Invalid email
Request ID: req_01JD8Q
```

Quote the request id when you contact support.

## Debugging

`--verbose` prints one line per HTTP attempt to stderr: method, path, status,
latency, request id and, for writes, the Idempotency-Key. `--show-headers`
adds each response's headers. Neither ever prints a request or response body
or a credential; `Authorization` and cookie headers are redacted.

```console
$ tmh learners retrieve sub_01JD80AB2C3D4E5F6G7H8J9K0M --verbose
POST /v1/auth/token 200 84ms request-id=req_01JD8P
GET /v1/auth/introspection 200 35ms request-id=req_01JD8Q
GET /v1/organizations/org_acme/learners/sub_01JD80AB2C3D4E5F6G7H8J9K0M 200 41ms request-id=req_01JD8R
```

## Configuration and state files

`tmh` writes only these files, in `~/.config/tmh` on every OS
(`$XDG_CONFIG_HOME/tmh` when that is set). Both are written atomically and
readable only by you.

| File | Holds |
|---|---|
| `config.toml` | Each profile's client id and Organization id. A client secret or local Signing secret is here only when the OS keyring is unusable. |
| `state.toml` | When the version check last ran, and where `events tail` stopped for each profile and Organization. |

Secrets live in the OS keyring under the service name `tmh`.

## Privacy

`tmh` sends no telemetry and never logs request or response bodies. It
connects only to:

1. the TeachMeHIPAA API, for the commands you run;
2. the version check's GitHub releases API, unless you turn it off;
3. what you ask for yourself: your `--forward-to` URL, which gets nothing
   but the forwarded Event.

The version check asks
`https://api.github.com/repos/TeachMeHIPAA/teachmehipaa-cli/releases/latest`
for the latest release at most once a day, while your command runs, and
prints one line on stderr if a newer version exists. It never goes through
the connection `tmh` uses for the API, and sends nothing but a `User-Agent`
of `teachmehipaa-cli/<version> go/<version> <os>/<arch>`, waits
at most one second, and can never fail your command. It is skipped when
stderr isn't a terminal, when `CI` is set, for completion scripts, and for
builds without a release version. Set `TMH_NO_UPDATE_CHECK=1` to turn it off.

## Shell completion

`tmh completion <shell>` prints a completion script for bash, zsh, fish or
PowerShell. Homebrew and the deb and rpm packages install completions for
bash, zsh and fish for you. Otherwise, load it from your shell's startup file:

```sh
source <(tmh completion bash)                          # ~/.bashrc
source <(tmh completion zsh)                           # ~/.zshrc
tmh completion fish | source                           # ~/.config/fish/config.fish
tmh completion powershell | Out-String | Invoke-Expression  # $PROFILE
```

`tmh completion <shell> --help` shows how to install it permanently.

## License

MIT. See [LICENSE](LICENSE). To report a vulnerability, see
[SECURITY.md](SECURITY.md).
